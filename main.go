package main

import (
	"time"

	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/cmd"
	"github.com/OdyseeTeam/odysee-api/internal/monitor"
	"github.com/OdyseeTeam/odysee-api/internal/storage"
	"github.com/OdyseeTeam/odysee-api/pkg/migrator"
	"github.com/OdyseeTeam/odysee-api/version"

	"github.com/getsentry/sentry-go"
)

func main() {
	monitor.IsProduction = config.IsProduction()
	monitor.ConfigureSentry(config.GetSentryDSN(), version.GetDevVersion(), monitor.LogMode())
	defer sentry.Flush(2 * time.Second)

	db, err := migrator.ConnectDB(migrator.DBConfigFromApp(config.GetDatabase()))
	if err != nil {
		panic(err)
	}
	defer db.Close()
	storage.SetDB(db)
	go storage.WatchMetrics(10 * time.Second)

	cmd.Execute()
}
