package router

import (
	"database/sql"
	"errors"
	"math/rand"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/models"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"

	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

var logger = monitor.NewModuleLogger("router")

const DefaultServer = "default"

type SDK struct {
	hardcoded    map[string]string
	servers      []*models.LbrynetServer
	lastDBAccess time.Time

	load   map[*models.LbrynetServer]uint64
	loadMu sync.RWMutex
}

func New(servers map[string]string) *SDK {
	r := &SDK{
		hardcoded: servers,
	}
	r.populateServers()
	return r
}

func (r *SDK) GetAll() []*models.LbrynetServer {
	r.populateServers()
	return r.servers
}

func (r *SDK) GetServer(walletID string) *models.LbrynetServer {
	r.populateServers()
	sdk := r.GetBalancedSDK()
	if walletID != "" {
		sdk = r.getSDKByWalletID(walletID)
	}

	if walletID != "" && sdk.Address == "" {
		logger.Log().Errorf("walletID [%s] is set but there is no server associated with it.", walletID)
		sdk = r.getDefault() //"UNKOWN_SERVER"
	}
	logger.Log().Tracef("From wallet id [%s] server [%s] is returned", walletID, sdk.Address)
	return sdk
}

func (r *SDK) getSDKByWalletID(walletID string) *models.LbrynetServer {
	r.populateServers()
	userID := getUserID(walletID)
	var user *models.User
	var err error
	if boil.GetDB() != nil {
		user, err = models.Users(qm.Load(models.UserRels.LbrynetServer), models.UserWhere.ID.EQ(userID)).OneG()
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			logger.Log().Errorf("Error getting user %d from db: %s", userID, err.Error())
		}
	}

	server := 0
	if user != nil && user.R != nil && user.R.LbrynetServer != nil {
		for i, s := range r.servers {
			if s.ID == user.R.LbrynetServer.ID {
				server = i
			}
		}
	} else {
		server = userID % 10 % len(r.servers)
	}

	return r.servers[server]

}

func getUserID(walletID string) int {
	regex, err := regexp.Compile(`(\d+)`)
	if err != nil {
		return 0
	}
	useridStr := regex.FindAllString(walletID, 1)
	if len(useridStr) == 0 {
		return 0
	}
	userID, err := strconv.ParseInt(useridStr[0], 10, 64)
	if err != nil {
		return 0
	}

	return int(userID)
}

func (r *SDK) GetBalancedSDK() *models.LbrynetServer {
	r.populateServers()
	return r.servers[rand.Intn(len(r.servers))]
}

func (r *SDK) populateServers() {
	if len(r.servers) > 0 && time.Since(r.lastDBAccess) > 1*time.Second {
		// don't hammer the DB
		return
	}

	var servers []*models.LbrynetServer
	hasDefault := false

	if r.hardcoded != nil && len(r.hardcoded) > 0 {
		for name, address := range r.hardcoded {
			if name == DefaultServer {
				hasDefault = true
			}
			servers = append(servers, &models.LbrynetServer{Name: name, Address: address})
		}
	} else {
		r.lastDBAccess = time.Now()
		serversDB, err := models.LbrynetServers().AllG()
		if err != nil {
			logger.Log().Error("Error retrieving lbrynet servers: ", err)
		}
		for _, s := range serversDB {
			if s == nil {
				continue // TODO: does this ever happen?
			}
			if s.Name == DefaultServer {
				hasDefault = true
			}
			servers = append(servers, s)
		}
	}

	if len(servers) == 0 {
		logger.Log().Fatal("Router created with NO servers!")
	}
	if !hasDefault {
		logger.Log().Fatal(`There is no default lbrynet server defined with key "` + DefaultServer + `"`)
		// TODO: is a default server required? maybe if one is not given, we set one to be the default
	}

	sort.Slice(servers, func(i, j int) bool { return servers[i].Name < servers[j].Name })
	r.servers = servers
}

func (r *SDK) getDefault() *models.LbrynetServer {
	r.populateServers()
	for _, s := range r.servers {
		if s.Name == DefaultServer {
			return s
		}
	}
	return nil
}

// WatchLoad keeps updating the metrics on the number of wallets loaded for each instance
func (r *SDK) WatchLoad() {
	ticker := time.NewTicker(2 * time.Minute)

	logger.Log().Infof("SDK router watching over %v instances", len(r.servers))
	r.populateServers()
	r.updateLoadAndMetrics()

	for {
		<-ticker.C
		r.populateServers()
		r.updateLoadAndMetrics()
	}
}

func (r *SDK) updateLoadAndMetrics() {
	if r.load == nil {
		r.load = make(map[*models.LbrynetServer]uint64)
	}
	for _, server := range r.GetAll() {
		metric := metrics.LbrynetWalletsLoaded.WithLabelValues(server.Address)
		walletList, err := ljsonrpc.NewClient(server.Address).WalletList("", 1, 1)
		if err != nil {
			logger.Log().Errorf("lbrynet instance %v is not responding: %v", server.Address, err)
			r.loadMu.Lock()
			delete(r.load, server)
			r.loadMu.Unlock()
			metric.Set(-1.0)
			// TODO: maybe mark this instance as unresponsive so new traffic is routed to other instances
		} else {
			r.loadMu.Lock()
			r.load[server] = walletList.TotalPages - 1
			r.loadMu.Unlock()
			metric.Set(float64(walletList.TotalPages - 1))
		}
	}
}

// LeastLoaded returns the least-loaded wallet
func (r *SDK) LeastLoaded() *models.LbrynetServer {
	var best *models.LbrynetServer
	var min uint64

	r.loadMu.RLock()
	defer r.loadMu.RUnlock()

	for server, numWallets := range r.load {
		if best == nil || numWallets < min {
			best = server
			min = numWallets
		}
	}

	return best
}
