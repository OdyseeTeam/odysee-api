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
	mu            sync.RWMutex
	servers       []*models.LbrynetServer
	defaultServer *models.LbrynetServer

	useDB        bool
	lastDBAccess time.Time

	loadMu sync.RWMutex
	load   map[*models.LbrynetServer]uint64
}

func New(servers map[string]string) *SDK {
	r := &SDK{}
	if servers != nil && len(servers) > 0 {
		s := make([]*models.LbrynetServer, len(servers))
		i := 0
		for name, address := range servers {
			s[i] = &models.LbrynetServer{Name: name, Address: address}
			i++
		}
		r.setServers(s)
	} else {
		r.useDB = true
		r.reloadServersFromDB()
	}
	return r
}

func (r *SDK) GetAll() []*models.LbrynetServer {
	r.reloadServersFromDB()
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.servers
}

func (r *SDK) GetServer(walletID string) *models.LbrynetServer {
	r.reloadServersFromDB()

	var sdk *models.LbrynetServer
	if walletID == "" {
		sdk = r.RandomServer()
	} else {
		sdk = r.serverForWallet(walletID)
		if sdk.Address == "" {
			logger.Log().Errorf("wallet [%s] is set but there is no server associated with it.", walletID)
			sdk = r.getDefaultServer()
		}
	}

	logger.Log().Tracef("Using [%s] server for wallet [%s]", sdk.Address, walletID)
	return sdk
}

func (r *SDK) serverForWallet(walletID string) *models.LbrynetServer {
	userID := getUserID(walletID)
	var user *models.User
	var err error
	if boil.GetDB() != nil {
		user, err = models.Users(qm.Load(models.UserRels.LbrynetServer), models.UserWhere.ID.EQ(userID)).OneG()
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			logger.Log().Errorf("Error getting user %d from db: %s", userID, err.Error())
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if user == nil || user.R == nil || user.R.LbrynetServer == nil {
		return r.servers[userID%10%len(r.servers)] // r.RandomServer()
	}

	for _, s := range r.servers {
		if s.ID == user.R.LbrynetServer.ID {
			return s
		}
	}

	logger.Log().Errorf("Server for user %d is set in db but is not in current servers list", userID)
	return r.servers[userID%10%len(r.servers)] // r.RandomServer()
}

func (r *SDK) RandomServer() *models.LbrynetServer {
	r.reloadServersFromDB()
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.servers[rand.Intn(len(r.servers))]
}

func (r *SDK) getDefaultServer() *models.LbrynetServer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.defaultServer
}

func (r *SDK) reloadServersFromDB() {
	if !r.useDB || time.Since(r.lastDBAccess) < 5*time.Second {
		// don't hammer the DB
		return
	}

	r.lastDBAccess = time.Now()
	servers, err := models.LbrynetServers().AllG()
	if err != nil {
		logger.Log().Error("Error retrieving lbrynet servers: ", err)
	}

	sort.Slice(servers, func(i, j int) bool { return servers[i].Name < servers[j].Name })
	r.setServers(servers)
}

func (r *SDK) setServers(servers []*models.LbrynetServer) {
	if len(servers) == 0 {
		logger.Log().Fatal("Setting servers to empty list")
		// TODO: fatal? really? maybe just don't update the servers in this case?
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.servers = servers
	for _, s := range r.servers {
		if s.Name == DefaultServer {
			r.defaultServer = s
			return
		}
	}
	// if we got here, there is no default server
	logger.Log().Fatal(`Must have a lbrynet server with name "` + DefaultServer + `"`)
	// TODO: fatal? really? maybe just don't update the servers in this case?
	// TODO: is a default server required? maybe if one is not given, we set one to be the default
}

// WatchLoad keeps updating the metrics on the number of wallets loaded for each instance
func (r *SDK) WatchLoad() {
	ticker := time.NewTicker(2 * time.Minute)

	logger.Log().Infof("SDK router watching over %v instances", len(r.servers))
	r.reloadServersFromDB()
	r.updateLoadAndMetrics()

	for {
		<-ticker.C
		r.reloadServersFromDB()
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

func getUserID(walletID string) int {
	userID, err := strconv.ParseInt(regexp.MustCompile(`\d+`).FindString(walletID), 10, 64)
	if err != nil {
		return 0
	}
	return int(userID)
}
