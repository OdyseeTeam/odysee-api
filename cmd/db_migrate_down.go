package cmd

import (
	"strconv"

	"github.com/lbryio/lbrytv/internal/storage"
	"github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(dbMigrateDown)
}

var dbMigrateDown = &cobra.Command{
	Use:   "db_migrate_down",
	Short: "Unapply database migrations (rewind back to the initial state)",
	Run: func(cmd *cobra.Command, args []string) {
		var nrMigrations int64
		if len(args) > 0 {
			var err error
			nrMigrations, err = strconv.ParseInt(args[0], 10, 32)
			if err != nil {
				logrus.Error("non integer passed as argument to migration")
			}
		}

		storage.Conn.MigrateDown(int(nrMigrations))
	},
}
