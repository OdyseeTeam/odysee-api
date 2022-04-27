package cmd

import (
	"strconv"

	"github.com/lbryio/lbrytv/apps/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/storage"
	"github.com/lbryio/lbrytv/pkg/migrator"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(dbMigrateUp)
}

var dbMigrateUp = &cobra.Command{
	Use:   "db_migrate_up",
	Short: "Apply database migrations",
	Run: func(cmd *cobra.Command, args []string) {
		var max int
		if len(args) > 0 {
			var err error
			max, err = strconv.Atoi(args[0])
			if err != nil {
				logrus.Error("non integer passed as argument to migration")
			}
		}

		dbConfig := config.GetDatabase()
		db, err := migrator.ConnectDB(migrator.DefaultDBConfig().DSN(dbConfig.Connection).Name(dbConfig.DBName).NoMigration(), storage.MigrationsFS)
		if err != nil {
			panic(err)
		}
		defer db.Close()
		m := migrator.New(db, storage.MigrationsFS)
		m.MigrateUp(max)
	},
}
