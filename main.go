package main

import (
	"math/rand"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"

	"github.com/lbryio/lbrytv/apps/lbrytv/config"
	"github.com/lbryio/lbrytv/cmd"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/internal/storage"
	"github.com/lbryio/lbrytv/version"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// this is a *client-side* timeout (for when we make http requests, not when we serve them)
	//https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
	http.DefaultClient.Timeout = 20 * time.Second

	defer func() {
		sentry.Flush(3 * time.Second)
		sentry.Recover()
	}()

	dbConfig := config.GetDatabase()
	monitor.IsProduction = config.IsProduction()
	monitor.ConfigureSentry(config.GetSentryDSN(), version.GetDevVersion(), monitor.LogMode())
	conn := storage.InitConn(storage.ConnParams{
		Connection: dbConfig.Connection,
		DBName:     dbConfig.DBName,
		Options:    dbConfig.Options,
	})

	err := conn.Connect()
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	conn.SetDefaultConnection()
	go conn.WatchMetrics(10 * time.Second)

	cmd.Execute()
}
