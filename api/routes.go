package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/lbryio/lbrytv/app/auth"
	"github.com/lbryio/lbrytv/app/proxy"
	"github.com/lbryio/lbrytv/app/publish"
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/internal/status"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var logger = monitor.NewModuleLogger("api")

// InstallRoutes sets up global API handlers
func InstallRoutes(r *mux.Router, sdkRouter *sdkrouter.Router) {
	upHandler := &publish.Handler{UploadPath: config.GetPublishSourceDir()}

	r.Use(methodTimer)

	r.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("lbrytv api"))
	})

	authProvider := auth.NewIAPIProvider(sdkRouter, config.GetInternalAPIHost())
	middlewareStack := middlewares(
		sdkrouter.Middleware(sdkRouter),
		auth.Middleware(authProvider),
	)

	v1Router := r.PathPrefix("/api/v1").Subrouter()
	v1Router.Use(middlewareStack)
	v1Router.HandleFunc("/proxy", proxy.HandleCORS).Methods(http.MethodOptions)
	v1Router.HandleFunc("/proxy", upHandler.Handle).MatcherFunc(upHandler.CanHandle)
	v1Router.HandleFunc("/proxy", proxy.Handle)
	v1Router.HandleFunc("/metric/ui", metrics.TrackUIMetric).Methods(http.MethodPost)
	v1Router.HandleFunc("/status", status.GetStatus)

	internalRouter := r.PathPrefix("/internal").Subrouter()
	internalRouter.Handle("/metrics", promhttp.Handler())
	internalRouter.Handle("/status", middlewareStack(http.HandlerFunc(status.GetStatus))) // deprecated. moved to /api/v1/status
	internalRouter.HandleFunc("/whoami", status.WhoAMI)
}

// applies several middleware in order
func middlewares(mws ...mux.MiddlewareFunc) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		for _, mw := range mws {
			next = mw(next)
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}

func methodTimer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next.ServeHTTP(w, r)

		path := r.URL.Path
		if r.URL.RawQuery != "" && !strings.HasPrefix(path, "/api/v1/metric") {
			path += "?" + r.URL.RawQuery
		}
		metrics.LbrytvCallDurations.WithLabelValues(path).Observe(time.Since(start).Seconds())
	})
}
