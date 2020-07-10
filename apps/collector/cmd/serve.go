package cmd

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/lbryio/lbrytv/apps/collector/collector"
	"github.com/lbryio/lbrytv/apps/environment"
	"github.com/lbryio/lbrytv/internal/storage"
	"github.com/lbryio/lbrytv/pkg/app"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "metrics",
	Short: "metrics is a metrics collection server for lbry.tv",
	Run: func(cmd *cobra.Command, args []string) {
		rand.Seed(time.Now().UnixNano()) // always seed random!

		e := environment.ForCollector()
		conn := e.Get("storage").(*storage.Connection)
		m := storage.NewMigrator(conn, "./apps/collector/migrations")
		m.MigrateUp()

		// db.SetDefaultConnection()

		app := app.New(":8080")
		app.InstallRoutes(collector.RouteInstaller)
		app.Start()
		app.ServeUntilShutdown()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
