package main

import (
	"math/rand"
	"time"

	"github.com/lbryio/lbrytv/apps/lbrytv/config"
	"github.com/lbryio/lbrytv/cmd"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/internal/storage"
	"github.com/lbryio/lbrytv/pkg/migrator"
	"github.com/lbryio/lbrytv/version"

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

	cmd.Execute()
}
