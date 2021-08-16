package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lbryio/lbrytv/api"
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/internal/monitor"

	"github.com/gorilla/mux"
)

var logger = monitor.NewModuleLogger("server")

// Server holds entities that can be used to control the web server
type Server struct {
	address  string
	listener *http.Server
	stopChan chan os.Signal
	stopWait time.Duration
}

// NewServer returns a server initialized with settings from supplied options.
func NewServer(address string, sdkRouter *sdkrouter.Router) *Server {
	r := mux.NewRouter()
	api.InstallRoutes(r, sdkRouter)
	r.Use(monitor.ErrorLoggingMiddleware)
	r.Use(defaultHeadersMiddleware(map[string]string{
		"Server":                       "api.lbry.tv",
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Headers": "content-type", // Needed this to get any request to work
	}))

	return &Server{
		address:  address,
		stopWait: 15 * time.Second,
		stopChan: make(chan os.Signal),
		listener: &http.Server{
			Addr:    address,
			Handler: r,
			// We need this for long uploads
			WriteTimeout: 0,
			// prev WriteTimeout was (sdkrouter.RPCTimeout + (1 * time.Second)).
			// It must be longer than rpc timeout to allow those timeouts to be handled
			IdleTimeout:       0,
			ReadHeaderTimeout: 10 * time.Second,
		},
	}
}

func defaultHeadersMiddleware(defaultHeaders map[string]string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for k, v := range defaultHeaders {
				w.Header().Set(k, v)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Address returns the address which server is listening on.
func (s *Server) Address() string {
	return s.address
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
	logger.Log().Infof("http server listening on %v", s.listener.Addr)
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
