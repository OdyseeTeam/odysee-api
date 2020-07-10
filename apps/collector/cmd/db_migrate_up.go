package cmd

import (
	"github.com/lbryio/lbrytv/apps/environment"
	"github.com/lbryio/lbrytv/internal/storage"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(dbMigrateUp)
}

var dbMigrateUp = &cobra.Command{
	Use:   "db_migrate_up",
	Short: "Apply unapplied database migrations",
	Run: func(cmd *cobra.Command, args []string) {
		e := environment.ForCollector()
		conn := e.Get("storage").(*storage.Connection)
		m := storage.NewMigrator(conn, "./apps/collector/migrations")
		m.MigrateUp()
	},
}
