package cmd

import (
	"github.com/lbryio/lbrytv/apps/collector/collector"
	"github.com/lbryio/lbrytv/apps/environment"
	"github.com/lbryio/lbrytv/internal/storage"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(dbMigrateDown)
}

var dbMigrateDown = &cobra.Command{
	Use:   "db_migrate_down",
	Short: "Migrate database schema down",
	Run: func(cmd *cobra.Command, args []string) {
		e := environment.ForCollector()
		conn := e.Get("storage").(*storage.Connection)
		collector.Migrator.MigrateDown(conn, 0)
	},
}
