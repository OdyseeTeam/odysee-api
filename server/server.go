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
	"github.com/lbryio/lbrytv/internal/environment"
	"github.com/lbryio/lbrytv/internal/monitor"

	"github.com/gorilla/mux"
)

// Server holds entities that can be used to control the web server
type Server struct {
	monitor.ModuleLogger

	InterruptChan  chan os.Signal
	DefaultHeaders map[string]string
	Environment    *environment.Env
	ProxyService   *proxy.Service

	address  string
	router   *mux.Router
	listener *http.Server
}

// ServerOpts holds basic web server settings
type ServerOpts struct {
	Address      string
	ProxyService *proxy.Service
}

// NewServer returns a server initialized with settings from global config.
func NewServer(opts ServerOpts) *Server {
	s := &Server{
		ModuleLogger:   monitor.NewModuleLogger("server"),
		InterruptChan:  make(chan os.Signal),
		DefaultHeaders: make(map[string]string),
		ProxyService:   opts.ProxyService,
		address:        opts.Address,
	}
	s.DefaultHeaders["Access-Control-Allow-Origin"] = "*"
	s.DefaultHeaders["Access-Control-Allow-Headers"] = "X-Lbry-Auth-Token, Origin, X-Requested-With, Content-Type, Accept"
	s.DefaultHeaders["Server"] = "api.lbry.tv"

	s.router = s.configureRouter()
	s.listener = s.configureListener()

	return s
}

func (s *Server) configureListener() *http.Server {
	return &http.Server{
		Addr:    s.address,
		Handler: s.router,
		// Can't have WriteTimeout set for streaming endpoints
		WriteTimeout: time.Second * 0,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
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
				s.Log().Info(err)
			} else {
				s.Log().Fatal(err)
			}
		}
	}()
	s.Log().Infof("http server listening on %v", s.address)
	return nil
}

// ServeUntilShutdown blocks until a shutdown signal is received, then shuts down the http server.
func (s *Server) ServeUntilShutdown() {
	signal.Notify(s.InterruptChan, os.Interrupt, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGINT)
	sig := <-s.InterruptChan
	s.Log().Printf("caught a signal (%v), shutting down http server...", sig)
	err := s.Shutdown()
	if err != nil {
		s.Log().Error("error shutting down server: ", err)
	} else {
		s.Log().Info("http server shut down")
	}
}

// Shutdown gracefully shuts down the peer server.
func (s *Server) Shutdown() error {
	err := s.listener.Shutdown(context.Background())
	return err
}
