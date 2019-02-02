package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/lbryio/lbryweb.go/config"
	"github.com/lbryio/lbryweb.go/monitor"
	"github.com/lbryio/lbryweb.go/routes"
	log "github.com/sirupsen/logrus"
)

// Serve is the main app entry point that configures and starts a webserver
func Serve() {
	staticDir := config.Settings.GetString("StaticDir")
	port := config.Settings.GetString("Port")

	// api.TraceEnabled = meta.Debugging

	// hs := make(map[string]string)
	// hs["Server"] = "lbry.tv" // TODO: change this to whatever it ends up being
	// hs["Content-Type"] = "application/json; charset=utf-8"
	// hs["Access-Control-Allow-Methods"] = "GET, POST, OPTIONS"
	// hs["Access-Control-Allow-Origin"] = "*"
	// hs["X-Content-Type-Options"] = "nosniff"
	// hs["X-Frame-Options"] = "deny"
	// hs["Content-Security-Policy"] = "default-src 'none'"
	// hs["X-XSS-Protection"] = "1; mode=block"
	// hs["Referrer-Policy"] = "same-origin"
	// if !meta.Debugging {
	// 	//hs["Strict-Transport-Security"] = "max-age=31536000; preload"
	// }
	// api.ResponseHeaders = hs

	router := mux.NewRouter()
	router.HandleFunc("/api/proxy", routes.Proxy)
	router.HandleFunc("/", routes.Index)
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))
	// httpServeMux.Handle("/", http.FileServer(&assetfs.AssetFS{Asset: assets.Asset, AssetDir: assets.AssetDir, AssetInfo: assets.AssetInfo, Prefix: "assets"}))
	router.Use(monitor.RequestLoggingMiddleware)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
		//https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
		//https://blog.cloudflare.com/exposing-go-on-the-internet/
		ReadTimeout: 5 * time.Second,
		//WriteTimeout: 10 * time.Second, // cant use this yet, since some of our responses take a long time (e.g. sending emails)
		IdleTimeout: 120 * time.Second,
		// ErrorLog:    logger.Writer,
	}

	go func() {
		log.Printf("listening on 0.0.0.0:%v", port)
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
