package main

//go:generate go-bindata -nometadata -o assets/bindata.go -pkg assets -ignore bindata.go assets/...
//go:generate go fmt ./assets/bindata.go

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/lbryio/lbryweb.go/assets"
	"github.com/lbryio/lbryweb.go/meta"
	"github.com/lbryio/lbryweb.go/routes"

	"github.com/lbryio/lbry.go/api"

	"github.com/elazarl/go-bindata-assetfs"
	"github.com/fatih/color"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cast"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})

	// this is a *client-side* timeout (for when we make http requests, not when we serve them)
	//https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
	http.DefaultClient.Timeout = 20 * time.Second

	if len(os.Args) < 2 {
		log.Errorf("Usage: %s COMMAND", os.Args[0])
		return
	}

	meta.Debugging = os.Getenv("DEBUGGING") == "1"
	if meta.Debugging {
		log.SetLevel(log.DebugLevel)
	}

	command := os.Args[1]
	switch command {
	case "version":
		fmt.Println(meta.GetVersion())
	case "serve":
		serve()
	default:
		log.Errorf("Invalid command: '%s'\n", command)
	}
}

func serve() {
	port := 8000

	api.TraceEnabled = meta.Debugging

	api.Log = func(request *http.Request, response *api.Response, err error) {
		consoleText := request.RemoteAddr + " [" + strconv.Itoa(response.Status) + "]: " + request.Method + " " + request.URL.Path
		if err == nil {
			log.Debug(color.GreenString(consoleText))
		} else {
			log.Error(color.RedString(consoleText + ": " + err.Error()))
		}
	}

	hs := make(map[string]string)
	hs["Server"] = "lbry.tv" // TODO: change this to whatever it ends up being
	hs["Content-Type"] = "application/json; charset=utf-8"
	hs["Access-Control-Allow-Methods"] = "GET, POST, OPTIONS"
	hs["Access-Control-Allow-Origin"] = "*"
	hs["X-Content-Type-Options"] = "nosniff"
	hs["X-Frame-Options"] = "deny"
	hs["Content-Security-Policy"] = "default-src 'none'"
	hs["X-XSS-Protection"] = "1; mode=block"
	hs["Referrer-Policy"] = "same-origin"
	if !meta.Debugging {
		//hs["Strict-Transport-Security"] = "max-age=31536000; preload"
	}
	api.ResponseHeaders = hs

	httpServeMux := http.NewServeMux()

	httpServeMux.Handle("/api", api.Handler(routes.Index))
	httpServeMux.Handle("/", http.FileServer(&assetfs.AssetFS{Asset: assets.Asset, AssetDir: assets.AssetDir, AssetInfo: assets.AssetInfo, Prefix: "assets"}))

	srv := &http.Server{
		Addr:    ":" + cast.ToString(port),
		Handler: http.Handler(httpServeMux),
		//https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
		//https://blog.cloudflare.com/exposing-go-on-the-internet/
		ReadTimeout: 5 * time.Second,
		//WriteTimeout: 10 * time.Second, // cant use this yet, since some of our responses take a long time (e.g. sending emails)
		IdleTimeout: 120 * time.Second,
	}

	log.Printf("Listening on port %v", port)
	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			//Normal graceful shutdown error
			if err.Error() == "http: Server closed" {
				log.Info(err)
			} else {
				log.Fatal(err)
			}
		}
	}()

	//Wait for shutdown signal, then shutdown api server. This will wait for all connections to finish.
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGINT)
	<-interruptChan
	log.Debug("Shutting down ...")
	err := srv.Shutdown(context.Background())
	if err != nil {
		log.Error("Error shutting down server: ", err)
	}
}
