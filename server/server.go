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

type Server struct {
	Config        *ServerConfig
	Logger        *log.Logger
	router        *mux.Router
	httpListener  *http.Server
	InterruptChan chan os.Signal
}

type ServerConfig struct {
	StaticDir string
	Address   string
}

// NewConfiguredServer returns a server initialized with settings from global config.
func NewConfiguredServer() *Server {
	return &Server{
		Config: &ServerConfig{
			StaticDir: config.Settings.GetString("StaticDir"),
			Address:   config.Settings.GetString("Address"),
		},
		Logger:        monitor.Logger,
		InterruptChan: make(chan os.Signal),
	}
}

func (s *Server) configureHTTPListener() *http.Server {
	return &http.Server{
		Addr:        s.Config.Address,
		Handler:     s.router,
		ReadTimeout: 5 * time.Second,
		// WriteTimeout: 30 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}
}

func configureRouter(staticDir string) *mux.Router {
	router := mux.NewRouter()

	router.HandleFunc("/", routes.Index)
	router.HandleFunc("/api/proxy", routes.Proxy)
	router.HandleFunc("/api/proxy/", routes.Proxy)
	router.HandleFunc("/content/claims/{uri}/{claim}/{filename}", routes.ContentByClaimsURI)
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

	router.Use(monitor.RequestLoggingMiddleware)
	return router
}

// Start starts a http server and returns immediately.
func (s *Server) Start() error {
	s.router = configureRouter(s.Config.StaticDir)
	s.Logger.Printf("serving %v at /static/", s.Config.StaticDir)
	s.httpListener = s.configureHTTPListener()

	go func() {
		err := s.httpListener.ListenAndServe()
		if err != nil {
			//Normal graceful shutdown error
			if err.Error() == "http: server closed" {
				s.Logger.Info(err)
			} else {
				s.Logger.Fatal(err)
			}
		}
	}()
	s.Logger.Printf("listening on %v", s.Config.Address)
	return nil
}

// WaitForShutdown blocks until a shutdown signal is received, then shuts down the http server.
func (s *Server) WaitForShutdown() {
	signal.Notify(s.InterruptChan, os.Interrupt, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGINT)
	sig := <-s.InterruptChan
	s.Logger.Printf("caught a signal (%v), shutting down http server...", sig)
	err := s.Shutdown()
	if err != nil {
		s.Logger.Error("error shutting down server: ", err)
	} else {
		s.Logger.Info("http server shut down")
	}
}

// Shutdown gracefully shuts down the peer server.
func (s *Server) Shutdown() error {
	err := s.httpListener.Shutdown(context.Background())
	return err
}

// ServeUntilInterrupted is the main module entry point that configures and starts a webserver,
// which runs until one of OS shutdown signals are received. The function is blocking.
func ServeUntilInterrupted() {
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

	server := NewConfiguredServer()
	err := server.Start()
	if err != nil {
		log.Fatal(err)
	}
	server.WaitForShutdown()
}
