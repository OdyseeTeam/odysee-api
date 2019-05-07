package main

import (
	"math/rand"
	"net/http"
	"os"
	"time"

	// "github.com/lbryio/lbrytv/assets"

	"github.com/lbryio/lbrytv/db"
	"github.com/lbryio/lbrytv/monitor"
	"github.com/lbryio/lbrytv/server"
	log "github.com/sirupsen/logrus"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	monitor.SetupLogging()

	// this is a *client-side* timeout (for when we make http requests, not when we serve them)
	//https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
	http.DefaultClient.Timeout = 20 * time.Second

	if len(os.Args) < 2 {
		log.Errorf("usage: %s COMMAND", os.Args[0])
		return
	}

	command := os.Args[1]
	switch command {
	case "version":
		log.Printf("lbrytv %v, commit %v, built at %v", version, commit, date)
	case "serve":
		log.Printf("lbrytv %v starting", version)
		server.ServeUntilInterrupted()
	case "db_migrate":
		log.Printf("lbrytv %v applying migrations", version)
		c := db.NewConnection(db.GetDefaultDSN())
		c.MigrateUp()
	case "db_migrate_down":
		log.Printf("lbrytv %v unapplying migrations", version)
		c := db.NewConnection(db.GetDefaultDSN())
		c.MigrateDown()
	default:
		log.Errorf("invalid command: '%s'\n", command)
	}
}
