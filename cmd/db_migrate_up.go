package cmd

import (
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
		storage.Init().MigrateUp()
	},
}
