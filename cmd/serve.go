package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/metrics_server"
	"github.com/lbryio/lbrytv/server"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "lbrytv",
	Short: "lbrytv is a backend API server for lbry.tv frontend",
	Run: func(cmd *cobra.Command, args []string) {
		s := server.NewServer(config.GetAddress())
		err := s.Start()
		if err != nil {
			log.Fatal(err)
		}
		ms := metrics_server.NewServer(config.MetricsAddress(), config.MetricsPath(), s.ProxyService)
		ms.Serve()
		s.ServeUntilShutdown()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
