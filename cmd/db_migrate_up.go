package cmd

import (
	"strconv"

	"github.com/lbryio/lbrytv/internal/storage"
	"github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(dbMigrateUp)
}

var dbMigrateUp = &cobra.Command{
	Use:   "db_migrate_up",
	Short: "Apply unapplied database migrations",
	Run: func(cmd *cobra.Command, args []string) {
		var nrMigrations int64
		if len(args) > 0 {
			var err error
			nrMigrations, err = strconv.ParseInt(args[0], 10, 32)
			if err != nil {
				logrus.Error("non integer passed as argument to migration")
			}
		}
		storage.Conn.MigrateUp(int(nrMigrations))
	},
}
