package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/lbryio/lbrytv/app/proxy"
	"github.com/lbryio/lbrytv/app/router"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/server"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "lbrytv",
	Short: "lbrytv is a backend API server for lbry.tv frontend",
	Run: func(cmd *cobra.Command, args []string) {
		s := server.NewServer(server.Options{
			Address:      config.GetAddress(),
			ProxyService: proxy.NewService(proxy.Opts{SDKRouter: router.New(config.GetLbrynetServers())}),
		})
		err := s.Start()
		if err != nil {
			log.Fatal(err)
		}

		// ServeUntilShutdown is blocking, should be last
		s.ServeUntilShutdown()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
