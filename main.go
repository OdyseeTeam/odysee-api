package main

import (
	"math/rand"
	"time"

	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/cmd"
	"github.com/OdyseeTeam/odysee-api/internal/monitor"
	"github.com/OdyseeTeam/odysee-api/internal/storage"
	"github.com/OdyseeTeam/odysee-api/pkg/migrator"
	"github.com/OdyseeTeam/odysee-api/pkg/redislocker"
	"github.com/OdyseeTeam/odysee-api/version"

	"github.com/getsentry/sentry-go"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	defer func() {
		sentry.Flush(3 * time.Second)
		sentry.Recover()
	}()

	monitor.IsProduction = config.IsProduction()
	monitor.ConfigureSentry(config.GetSentryDSN(), version.GetDevVersion(), monitor.LogMode())

	db, err := migrator.ConnectDB(migrator.DBConfigFromApp(config.GetDatabase()).NoMigration(), storage.MigrationsFS)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	storage.SetDB(db)
	go storage.WatchMetrics(10 * time.Second)

	redislocker.RegisterMetrics()

	cmd.Execute()
}
