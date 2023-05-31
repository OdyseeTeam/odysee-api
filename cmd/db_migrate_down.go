package cmd

import (
	"strconv"

	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/storage"
	"github.com/OdyseeTeam/odysee-api/pkg/migrator"
	"github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(dbMigrateDown)
}

var dbMigrateDown = &cobra.Command{
	Use:   "db_migrate_down",
	Short: "Unapply database migrations",
	Run: func(_ *cobra.Command, args []string) {
		var max int
		if len(args) > 0 {
			var err error
			max, err = strconv.Atoi(args[0])
			if err != nil {
				logrus.Error("non integer passed as argument to migration")
			}
		}

		dbConfig := config.GetDatabase()
		db, err := migrator.ConnectDB(migrator.DefaultDBConfig().DSN(dbConfig.Connection).Name(dbConfig.DBName))
		if err != nil {
			panic(err)
		}
		defer db.Close()
		m := migrator.New(db, storage.MigrationsFS)
		_, err = m.MigrateDown(max)
		if err != nil {
			panic(err)
		}
	},
}
