package main

import (
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/lbryio/lbrytv/db"
	"github.com/lbryio/lbrytv/server"
	"github.com/lbryio/lbrytv/version"

	log "github.com/sirupsen/logrus"
)

func main() {
	rand.Seed(time.Now().UnixNano())

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
		log.Print("lbrytv %v", version.GetFullBuildName())
	case "serve":
		log.Printf("lbrytv %v starting", version.GetDevVersion())
		server.ServeUntilInterrupted()
	case "db_migrate_up":
		log.Printf("lbrytv %v applying migrations", version.GetDevVersion())
		db.Init().MigrateUp()
	case "db_migrate_down":
		log.Printf("lbrytv %v unapplying migrations", version.GetDevVersion())
		db.Init().MigrateDown()
	default:
		log.Errorf("invalid command: '%s'\n", command)
	}
}
