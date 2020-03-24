package status

import (
	"math/rand"
	"time"

	"github.com/lbryio/lbrytv/app/router"
	"github.com/lbryio/lbrytv/internal/metrics"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
)

func WatchWallets() {
	r := router.NewDefault()
	Logger.Log().Infof("starting wallets watcher over %v instances", len(r.GetSDKServerList()))
	walletWatchInterval := time.Duration(rand.Intn(10)+5) * time.Minute
	ticker := time.NewTicker(walletWatchInterval)

	go func() {
		countWallets()
		for {
			<-ticker.C
			countWallets()
		}
	}()
}

func countWallets() {
	r := router.NewDefault()
	for _, server := range r.GetSDKServerList() {
		c := ljsonrpc.NewClient(server.Address)
		m := metrics.LbrynetWalletsLoaded.WithLabelValues(server.Address)

		wl, err := c.WalletList("", 1, 1)
		if err != nil {
			Logger.Log().Errorf("lbrynet instance %v is not responding: %v", server.Address, err)
			m.Set(0.0)
		} else {
			m.Set(float64(wl.TotalPages - 1))
		}
	}
}
