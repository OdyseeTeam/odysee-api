package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lbryio/lbrytv/api"
	"github.com/lbryio/lbrytv/app/proxy"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/environment"
	"github.com/lbryio/lbrytv/internal/metrics_server"
	"github.com/lbryio/lbrytv/internal/monitor"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

// Server holds entities that can be used to control the web server
type Server struct {
	Config         *Config
	Logger         *log.Logger
	router         *mux.Router
	listener       *http.Server
	InterruptChan  chan os.Signal
	DefaultHeaders map[string]string
	Environment    *environment.Env
	MetricsServer  *metrics_server.Server
	ProxyService   *proxy.Service
}

// Config holds basic web server settings
type Config struct {
	Address string
}

// NewConfiguredServer returns a server initialized with settings from global config.
func NewConfiguredServer() *Server {
	s := &Server{
		Config: &Config{
			Address: config.GetAddress(),
		},
		Logger:         monitor.Logger,
		InterruptChan:  make(chan os.Signal),
		DefaultHeaders: make(map[string]string),
		ProxyService:   proxy.NewService(),
	}
	s.DefaultHeaders["Access-Control-Allow-Origin"] = "*"
	s.DefaultHeaders["Access-Control-Allow-Headers"] = "X-Lbry-Auth-Token, Origin, X-Requested-With, Content-Type, Accept"
	s.DefaultHeaders["Server"] = "api.lbry.tv"

	s.router = s.configureRouter()
	s.listener = s.configureListener()

	ms := metrics_server.NewServer(s.ProxyService)
	ms.Serve()

	return s
}

func (s *Server) configureListener() *http.Server {
	return &http.Server{
		Addr:        s.Config.Address,
		Handler:     s.router,
		ReadTimeout: 5 * time.Second,
		// WriteTimeout: 30 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}
}

func (s *Server) defaultHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range s.DefaultHeaders {
			w.Header().Set(k, v)
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) configureRouter() *mux.Router {
	r := mux.NewRouter()

	api.InstallRoutes(s.ProxyService, r)

	r.Use(monitor.RequestLoggingMiddleware)
	r.Use(s.defaultHeadersMiddleware)
	return r
}

// Start starts a http server and returns immediately.
func (s *Server) Start() error {
	go func() {
		err := s.listener.ListenAndServe()
		if err != nil {
			// Normal graceful shutdown error
			if err.Error() == "http: Server closed" {
				s.Logger.Info(err)
			} else {
				s.Logger.Fatal(err)
			}
		}
	}()
	s.Logger.Printf("listening on %v", s.Config.Address)
	return nil
}

// ServeUntilShutdown blocks until a shutdown signal is received, then shuts down the http server.
func (s *Server) ServeUntilShutdown() {
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
	err := s.listener.Shutdown(context.Background())
	return err
}

// ServeUntilInterrupted is the main module entry point that configures and starts a webserver,
// which runs until one of OS shutdown signals are received. The function is blocking.
func ServeUntilInterrupted() {
	s := NewConfiguredServer()
	s.Logger.Info("http server starting...")
	err := s.Start()
	if err != nil {
		log.Fatal(err)
	}
	s.Logger.Infof("http server listening on %v", s.Config.Address)
	s.ServeUntilShutdown()
}
