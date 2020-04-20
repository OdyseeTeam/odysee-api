package sdkrouter

import (
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/models"
	"github.com/sirupsen/logrus"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
)

var logger = monitor.NewModuleLogger("sdkrouter")

func DisableLogger() { logger.Disable() } // for testing

type Router struct {
	mu      sync.RWMutex
	servers []*models.LbrynetServer

	loadMu sync.RWMutex
	load   map[*models.LbrynetServer]uint64

	useDB      bool
	lastLoaded time.Time
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
	logger.WithFields(logrus.Fields{"lock": "mu"}).Trace("waiting for read lock in GetAll")
	r.mu.RLock()
	logger.WithFields(logrus.Fields{"lock": "mu"}).Trace("got read lock in GetAll")
	defer r.mu.RUnlock()
	return r.servers
}

func (r *Router) RandomServer() *models.LbrynetServer {
	r.reloadServersFromDB()
	logger.WithFields(logrus.Fields{"lock": "mu"}).Trace("waiting for read lock in RandomServer")
	r.mu.RLock()
	logger.WithFields(logrus.Fields{"lock": "mu"}).Trace("got read lock in RandomServer")
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
		logger.Log().Error("Setting servers to empty list")
		return
	}

	// we do this partially to make sure that ids are assigned to servers more consistently,
	// and partially to make tests consistent (since Go maps are not ordered)
	sort.Slice(servers, func(i, j int) bool { return servers[i].Name < servers[j].Name })
	logger.WithFields(logrus.Fields{"lock": "mu"}).Trace("waiting for write lock in setServers")
	r.mu.Lock()
	logger.WithFields(logrus.Fields{"lock": "mu"}).Trace("got write lock in setServers")
	defer r.mu.Unlock()
	r.servers = servers
	logger.Log().Debugf("updated server list to %d servers", len(r.servers))
}

// WatchLoad keeps updating the metrics on the number of wallets loaded for each instance
func (r *Router) WatchLoad() {
	ticker := time.NewTicker(2 * time.Minute)

	logger.Log().Infof("SDK router watching load on %d instances", len(r.servers))
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
			logger.Log().Errorf("lbrynet instance %s is not responding: %v", server.Address, err)
			logger.WithFields(logrus.Fields{"lock": "loadMu"}).Trace("waiting for write lock in updateLoadAndMetrics 1")
			r.loadMu.Lock()
			logger.WithFields(logrus.Fields{"lock": "loadMu"}).Trace("got write lock in updateLoadAndMetrics 1")
			delete(r.load, server)
			r.loadMu.Unlock()
			metric.Set(-1.0)
			// TODO: maybe mark this instance as unresponsive so new users are assigned to other instances
		} else {
			logger.WithFields(logrus.Fields{"lock": "loadMu"}).Trace("waiting for write lock in updateLoadAndMetrics 2")
			r.loadMu.Lock()
			logger.WithFields(logrus.Fields{"lock": "loadMu"}).Trace("got write lock in updateLoadAndMetrics 2")
			r.load[server] = walletList.TotalPages
			r.loadMu.Unlock()
			metric.Set(float64(walletList.TotalPages))
		}
	}
	leastLoaded := r.LeastLoaded()
	logger.Log().Infof("After updating load, least loaded server is %s", leastLoaded.Address)
}

// LeastLoaded returns the least-loaded wallet
func (r *Router) LeastLoaded() *models.LbrynetServer {
	var best *models.LbrynetServer
	var min uint64

	logger.WithFields(logrus.Fields{"lock": "loadMu"}).Trace("waiting for read lock in LeastLoaded")
	r.loadMu.RLock()
	logger.WithFields(logrus.Fields{"lock": "loadMu"}).Trace("got read lock in LeastLoaded")
	defer r.loadMu.RUnlock()

	if len(r.load) == 0 {
		// updateLoadAndMetrics() was never run, so return a random server
		logger.Log().Warnf("LeastLoaded() called before updating load metrics. Returning random server.")
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

// WalletID formats user ID to use as an LbrynetServer wallet ID.
func WalletID(userID int) string {
	// warning: changing this template will require renaming the stored wallet files in lbrytv
	const template = "lbrytv-id.%d.wallet"
	return fmt.Sprintf(template, userID)
}
