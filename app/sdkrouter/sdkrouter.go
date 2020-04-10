package sdkrouter

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

type Router struct {
	mu      sync.RWMutex
	servers []*models.LbrynetServer

	loadMu sync.RWMutex
	load   map[*models.LbrynetServer]uint64

	useDB      bool
	lastLoaded time.Time
	rpcClient  *ljsonrpc.Client
}

func New(servers map[string]string) *Router {
	r := &Router{
		load: make(map[*models.LbrynetServer]uint64),
	}
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

func (r *Router) GetAll() []*models.LbrynetServer {
	r.reloadServersFromDB()
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.servers
}

func (r *Router) GetServer(walletID string) *models.LbrynetServer {
	r.reloadServersFromDB()

	var sdk *models.LbrynetServer
	if walletID == "" {
		sdk = r.LeastLoaded()
	} else {
		sdk = r.serverForWallet(walletID)
		if sdk.Address == "" {
			logger.Log().Errorf("wallet [%s] is set but there is no server associated with it.", walletID)
			sdk = r.RandomServer()
		}
	}

	logger.Log().Tracef("Using [%s] server for wallet [%s]", sdk.Address, walletID)
	return sdk
}

func (r *Router) serverForWallet(walletID string) *models.LbrynetServer {
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
		return r.servers[getServerForUserID(userID, len(r.servers))]
	}

	for _, s := range r.servers {
		if s.ID == user.R.LbrynetServer.ID {
			return s
		}
	}

	logger.Log().Errorf("Server for user %d is set in db but is not in current servers list", userID)
	return r.servers[getServerForUserID(userID, len(r.servers))]
}

func (r *Router) RandomServer() *models.LbrynetServer {
	r.reloadServersFromDB()
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.servers[rand.Intn(len(r.servers))]
}

func (r *Router) reloadServersFromDB() {
	if !r.useDB || time.Since(r.lastLoaded) < 5*time.Second {
		// don't hammer the DB
		return
	}

	r.lastLoaded = time.Now()
	servers, err := models.LbrynetServers().AllG()
	if err != nil {
		logger.Log().Error("Error retrieving lbrynet servers: ", err)
	}

	r.setServers(servers)
}

func (r *Router) setServers(servers []*models.LbrynetServer) {
	if len(servers) == 0 {
		logger.Log().Fatal("Setting servers to empty list")
		// TODO: fatal? really? maybe just don't update the servers in this case?
	}

	// we do this partially to make sure that ids are assigned to servers more consistently,
	// and partially to make tests consistent (since Go maps are not ordered)
	sort.Slice(servers, func(i, j int) bool { return servers[i].Name < servers[j].Name })
	r.mu.Lock()
	defer r.mu.Unlock()
	r.servers = servers
}

// WatchLoad keeps updating the metrics on the number of wallets loaded for each instance
func (r *Router) WatchLoad() {
	ticker := time.NewTicker(2 * time.Minute)

	logger.Log().Infof("Router router watching over %v instances", len(r.servers))
	r.reloadServersFromDB()
	r.updateLoadAndMetrics()

	for {
		<-ticker.C
		r.reloadServersFromDB()
		r.updateLoadAndMetrics()
	}
}

func (r *Router) updateLoadAndMetrics() {
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
			r.load[server] = walletList.TotalPages
			r.loadMu.Unlock()
			metric.Set(float64(walletList.TotalPages))
		}
	}
}

// LeastLoaded returns the least-loaded wallet
func (r *Router) LeastLoaded() *models.LbrynetServer {
	var best *models.LbrynetServer
	var min uint64

	r.loadMu.RLock()
	defer r.loadMu.RUnlock()

	if len(r.load) == 0 {
		// updateLoadAndMetrics() was never run, so return a random server
		return r.RandomServer()
	}

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

func getServerForUserID(userID, numServers int) int {
	return userID % numServers
}
