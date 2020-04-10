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
	"github.com/lbryio/lbrytv/internal/monitor"

	"github.com/gorilla/mux"
)

var logger = monitor.NewModuleLogger("server")

// Server holds entities that can be used to control the web server
type Server struct {
	defaultHeaders map[string]string
	proxyService   *proxy.ProxyService
	stopChan       chan os.Signal
	stopWait       time.Duration
	address        string
	router         *mux.Router
	listener       *http.Server
}

// Options holds basic web server settings.
type Options struct {
	Address         string
	ProxyService    *proxy.ProxyService
	StopWaitSeconds int
}

// NewServer returns a server initialized with settings from supplied options.
func NewServer(opts Options) *Server {
	s := &Server{
		proxyService: opts.ProxyService,
		address:      opts.Address,
		stopWait:     15 * time.Second,
		stopChan:     make(chan os.Signal),
		defaultHeaders: map[string]string{
			"Server":                      "api.lbry.tv",
			"Access-Control-Allow-Origin": "*",
		},
	}
	if opts.StopWaitSeconds != 0 {
		s.stopWait = time.Duration(opts.StopWaitSeconds) * time.Second
	}

	r := mux.NewRouter()
	api.InstallRoutes(s.proxyService, r)
	r.Use(monitor.ErrorLoggingMiddleware)
	r.Use(s.defaultHeadersMiddleware)
	s.router = r

	s.listener = &http.Server{
		Addr:    s.address,
		Handler: s.router,
		// Can't have WriteTimeout set for streaming endpoints
		WriteTimeout:      0,
		IdleTimeout:       0,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return s
}

func (s *Server) defaultHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range s.defaultHeaders {
			w.Header().Set(k, v)
		}
		next.ServeHTTP(w, r)
	})
}

// Start starts a http server and returns immediately.
func (s *Server) Start() error {
	go func() {
		if err := s.listener.ListenAndServe(); err != nil {
			if err.Error() != "http: Server closed" {
				logger.Log().Error(err)
			}
		}
	}()
	logger.Log().Infof("http server listening on %v", s.address)
	return nil
}

// ServeUntilShutdown blocks until a shutdown signal is received, then shuts down the http server.
func (s *Server) ServeUntilShutdown() {
	signal.Notify(s.stopChan, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGINT)
	sig := <-s.stopChan

	logger.Log().Printf("caught a signal (%v), shutting down http server...", sig)

	err := s.Shutdown()
	if err != nil {
		logger.Log().Error("error shutting down server: ", err)
	} else {
		logger.Log().Info("http server shut down")
	}
}

// Shutdown gracefully shuts down the peer server.
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.stopWait)
	defer cancel()
	err := s.listener.Shutdown(ctx)
	return err
}
