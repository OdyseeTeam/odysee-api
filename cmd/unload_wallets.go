package cmd

import (
	"os"
	"strconv"
	"time"

	"github.com/lbryio/lbrytv/app/wallet/tracker"
	"github.com/lbryio/lbrytv/internal/monitor"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/volatiletech/sqlboiler/boil"
)

func init() {
	rootCmd.AddCommand(unloadWallets)
}

var unloadWallets = &cobra.Command{
	Use:   "unload_wallets MIN",
	Short: "Unload wallets that have not been used in the last MIN minutes",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		min, err := strconv.Atoi(args[1])
		if err != nil {
			log.Error(args[1] + " is not an integer")
			os.Exit(1)
		}

		unloadOlderThan := time.Duration(min) * time.Minute
		_, err = tracker.Unload(boil.GetDB(), unloadOlderThan)
		if err != nil {
			log.Error(err)
			monitor.ErrorToSentry(err)
			os.Exit(1)
		}
	},
}
