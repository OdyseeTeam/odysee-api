package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/monitor"
	"github.com/lbryio/lbrytv/routes"
	log "github.com/sirupsen/logrus"
)

// Server holds entities that can be used to control the web server
type Server struct {
	Config         *Config
	Logger         *log.Logger
	router         *mux.Router
	httpListener   *http.Server
	InterruptChan  chan os.Signal
	DefaultHeaders map[string]string
}

// Config holds basic web server settings
type Config struct {
	StaticDir string
	Address   string
}

// NewConfiguredServer returns a server initialized with settings from global config.
func NewConfiguredServer() *Server {
	s := &Server{
		Config: &Config{
			StaticDir: config.Settings.GetString("StaticDir"),
			Address:   config.Settings.GetString("Address"),
		},
		Logger:         monitor.Logger,
		InterruptChan:  make(chan os.Signal),
		DefaultHeaders: make(map[string]string),
	}
	s.DefaultHeaders["Access-Control-Allow-Origin"] = "*"
	s.DefaultHeaders["Access-Control-Allow-Headers"] = "X-Lbry-Auth-Token, Origin, X-Requested-With, Content-Type, Accept"
	s.DefaultHeaders["Server"] = "lbrytv"
	return s
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

	routes.InstallRoutes(r)
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(s.Config.StaticDir))))

	r.Use(monitor.RequestLoggingMiddleware)
	r.Use(s.defaultHeadersMiddleware)
	return r
}

// Start starts a http server and returns immediately.
func (s *Server) Start() error {
	s.router = s.configureRouter()
	s.Logger.Printf("serving %v at /static/", s.Config.StaticDir)
	s.httpListener = s.configureHTTPListener()

	go func() {
		err := s.httpListener.ListenAndServe()
		if err != nil {
			//Normal graceful shutdown error
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
	err := s.httpListener.Shutdown(context.Background())
	return err
}

// ServeUntilInterrupted is the main module entry point that configures and starts a webserver,
// which runs until one of OS shutdown signals are received. The function is blocking.
func ServeUntilInterrupted() {
	server := NewConfiguredServer()
	err := server.Start()
	if err != nil {
		log.Fatal(err)
	}
	server.ServeUntilShutdown()
}
