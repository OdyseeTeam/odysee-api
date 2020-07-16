package app

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lbryio/lbrytv-player/pkg/logger"
	env "github.com/lbryio/lbrytv/apps/environment"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// App holds entities that can be used to control the web server
type App struct {
	Router      *mux.Router
	Address     string
	Environment *env.Environment

	logger   *logrus.Logger
	headers  map[string]string
	stopChan chan os.Signal
	stopWait time.Duration
	server   *http.Server
}

type RouteInstaller func(*mux.Router, *env.Environment)

// Options holds basic web service settings.
type Options struct {
	Environment       env.Environment
	Headers           map[string]string
	StopWait          time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	Logger            *logrus.Logger
}

type Option func(*Options)

func Header(key, value string) Option {
	return func(args *Options) {
		args.Headers[key] = value
	}
}

func StopWait(t time.Duration) Option {
	return func(args *Options) {
		args.StopWait = t
	}
}

func AllowOrigin(o string) Option {
	return func(args *Options) {
		args.Headers["Access-Control-Allow-Origin"] = o
		if o == "*" {
			args.Headers["Vary"] = "Origin"
		}
	}
}

func Logger(l *logrus.Logger) Option {
	return func(args *Options) {
		args.Logger = l
	}
}

func Environment(e env.Environment) Option {
	return func(args *Options) {
		args.Environment = e
	}
}

// New returns a new App HTTP server initialized with settings from supplied Opts.
func New(address string, setters ...Option) *App {
	args := &Options{
		Headers: map[string]string{
			"Server": "lbrytv app",
		},
		StopWait:          time.Second * 15,
		WriteTimeout:      time.Second * 5,
		IdleTimeout:       time.Second * 10,
		ReadHeaderTimeout: time.Second * 5,
		Logger:            logger.GetLogger(),
	}
	for _, setter := range setters {
		setter(args)
	}

	router := mux.NewRouter()

	app := &App{
		Address:     address,
		Router:      router,
		Environment: &args.Environment,
		logger:      args.Logger,
		headers:     args.Headers,
		stopWait:    args.StopWait,
		stopChan:    make(chan os.Signal),
		server: &http.Server{
			Addr:              address,
			Handler:           router,
			WriteTimeout:      args.WriteTimeout,
			IdleTimeout:       args.IdleTimeout,
			ReadHeaderTimeout: args.ReadHeaderTimeout,
		},
	}
	router.Use(app.headersMiddleware)

	return app
}

func (a *App) headersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range a.headers {
			w.Header().Set(k, v)
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) InstallRoutes(f RouteInstaller) {
	f(a.Router, a.Environment)
}

// Start starts a HTTP server and returns immediately.
func (a *App) Start() {
	go func() {
		a.logger.Infof("starting app server on %v", a.Address)
		if err := a.server.ListenAndServe(); err != nil {
			if err.Error() != "http: Server closed" {
				a.logger.Fatal(err)
			}
		}
	}()
}

// ServeUntilShutdown blocks until a shutdown signal is received, then shuts down the HTTP server.
func (a *App) ServeUntilShutdown() {
	signal.Notify(a.stopChan, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGINT)
	sig := <-a.stopChan
	a.logger.Printf("caught a signal (%v), shutting down http server...", sig)
	err := a.Shutdown()
	if err != nil {
		a.logger.Error("error shutting down http server: ", err)
	} else {
		a.logger.Info("http server shut down")
	}
}

// Shutdown gracefully shuts down the HTTP server.
func (a *App) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), a.stopWait)
	defer cancel()
	err := a.server.Shutdown(ctx)
	return err
}
