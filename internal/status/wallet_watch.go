package status

import (
	"time"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/metrics"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
)

const walletWatchInterval = time.Minute * 1

func WatchWallets() {
	StatusLogger.Log().Info("starting wallets watcher")
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
	servers := config.GetLbrynetServers()
	for _, server := range servers {
		c := ljsonrpc.NewClient(server)
		m := metrics.LbrynetWalletsLoaded.WithLabelValues(server)

		wl, err := c.WalletList("", 1, 1)
		if err != nil {
			StatusLogger.Log().Errorf("lbrynet instance %v is not responding", server)
			m.Set(0.0)
		} else {
			m.Set(float64(wl.TotalPages - 1))
		}
	}
}
