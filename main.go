package main

import (
	"math/rand"
	"net/http"
	"os"
	"time"

	// "github.com/lbryio/lbryweb.go/assets"

	"github.com/lbryio/lbryweb.go/config"
	"github.com/lbryio/lbryweb.go/server"
	"github.com/lbryio/lbryweb.go/updater"
	log "github.com/sirupsen/logrus"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// this is a *client-side* timeout (for when we make http requests, not when we serve them)
	//https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
	http.DefaultClient.Timeout = 20 * time.Second

	if len(os.Args) < 2 {
		log.Errorf("Usage: %s COMMAND", os.Args[0])
		return
	}

	command := os.Args[1]
	switch command {
	case "version":
		log.Printf("lbryweb %v, commit %v, built at %v", version, commit, date)
	case "serve":
		log.Printf("lbryweb %v starting", version)
		server.Serve()
	case "update_js":
		updater.GetLatestRelease("sayplastic/lbryweb-js", config.Settings.GetString("StaticDir"))
	default:
		log.Errorf("Invalid command: '%s'\n", command)
	}
}
