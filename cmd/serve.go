package cmd

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/OdyseeTeam/odysee-api/api"
	"github.com/OdyseeTeam/odysee-api/app/sdkrouter"
	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/server"
	"github.com/OdyseeTeam/player-server/pkg/paid"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "oapi",
	Short: "backend server for Odysee frontend",
	Run: func(_ *cobra.Command, _ []string) {
		rand.Seed(time.Now().UnixNano()) // always seed random!
		sdkRouter := sdkrouter.New(config.GetLbrynetServers())
		go sdkRouter.WatchLoad()

		s := server.NewServer(config.GetAddress(), sdkRouter, &api.RoutesOptions{
			EnableProfiling: config.GetProfiling(),
			EnableV3Publish: true,
		})
		err := s.Start()
		if err != nil {
			log.Fatal(err)
		}

		key, err := os.ReadFile(config.GetPaidTokenPrivKey())
		if err != nil {
			log.Fatal(err)
		}
		err = paid.InitPrivateKey(key)
		if err != nil {
			log.Fatal(err)
		}
		c := wallet.NewTokenCache()
		wallet.SetTokenCache(c)

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
