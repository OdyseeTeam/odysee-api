package cmd

import (
	db "github.com/lbryio/lbrytv/internal/storage"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(dbMigrateDown)
}

var dbMigrateDown = &cobra.Command{
	Use:   "db_migrate_down",
	Short: "Unapply database migrations (rewind back to the initial state)",
	Run: func(cmd *cobra.Command, args []string) {
		db.Init().MigrateDown()
	},
}
