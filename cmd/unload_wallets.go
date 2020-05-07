package cmd

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lbryio/lbrytv/app/wallet/accesstracker"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/volatiletech/sqlboiler/boil"
)

func init() {
	rootCmd.AddCommand(unloadWallets)
}

var unloadWallets = &cobra.Command{
	Use:   "unload_wallets",
	Short: "Unload wallets that have not been used recently",
	Run: func(cmd *cobra.Command, args []string) {
		// these could become args in the future
		runInterval := 1 * time.Hour
		unloadOlderThan := 4 * time.Hour

		stop := make(chan os.Signal)
		signal.Notify(stop, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGINT)

		unload(unloadOlderThan)

		t := time.NewTicker(runInterval)
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				unload(unloadOlderThan)
			}
		}
	},
}

func unload(olderThan time.Duration) {
	_, err := accesstracker.Unload(boil.GetDB(), olderThan)
	if err != nil {
		log.Error(err)
	}
}
