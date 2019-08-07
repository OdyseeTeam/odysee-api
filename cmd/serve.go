package cmd

import (
	"fmt"
	"os"

	"github.com/lbryio/lbrytv/server"
	"github.com/lbryio/lbrytv/internal/metrics"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "lbrytv",
	Short: "lbrytv is a backend API server for lbry.tv frontend",
	Run: func(cmd *cobra.Command, args []string) {
		metrics.Serve()
		server.ServeUntilInterrupted()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
