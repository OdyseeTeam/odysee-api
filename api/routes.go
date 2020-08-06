package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/lbryio/lbrytv-player/pkg/paid"
	"github.com/lbryio/lbrytv/app/auth"
	"github.com/lbryio/lbrytv/app/proxy"
	"github.com/lbryio/lbrytv/app/publish"
	"github.com/lbryio/lbrytv/app/query/cache"
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/app/wallet/tracker"
	"github.com/lbryio/lbrytv/apps/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/ip"
	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/middleware"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/internal/status"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/volatiletech/sqlboiler/boil"
)

var logger = monitor.NewModuleLogger("api")

// InstallRoutes sets up global API handlers
func InstallRoutes(r *mux.Router, sdkRouter *sdkrouter.Router) {
	upHandler := &publish.Handler{UploadPath: config.GetPublishSourceDir()}

	r.Use(methodTimer)

	r.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("lbrytv api"))
	})
	r.HandleFunc("", proxy.HandleCORS)

	v1Router := r.PathPrefix("/api/v1").Subrouter()
	v1Router.Use(defaultMiddlewares(sdkRouter, config.GetInternalAPIHost()))

	v1Router.HandleFunc("/proxy", upHandler.Handle).MatcherFunc(upHandler.CanHandle)
	v1Router.HandleFunc("/proxy", proxy.Handle).Methods(http.MethodPost)
	v1Router.HandleFunc("/proxy", proxy.HandleCORS).Methods(http.MethodOptions)

	v1Router.HandleFunc("/metric/ui", metrics.TrackUIMetric).Methods(http.MethodPost)
	v1Router.HandleFunc("/metric/ui", proxy.HandleCORS).Methods(http.MethodOptions)

	v1Router.HandleFunc("/status", status.GetStatus).Methods(http.MethodGet)
	v1Router.HandleFunc("/paid/pubkey", paid.HandlePublicKeyRequest).Methods(http.MethodGet)

	internalRouter := r.PathPrefix("/internal").Subrouter()
	internalRouter.Handle("/metrics", promhttp.Handler())

	v2Router := r.PathPrefix("/api/v2").Subrouter()
	v2Router.HandleFunc("/status", status.GetStatusV2).Methods(http.MethodGet)
	v2Router.HandleFunc("/status", proxy.HandleCORS).Methods(http.MethodOptions)
}

func defaultMiddlewares(rt *sdkrouter.Router, internalAPIHost string) mux.MiddlewareFunc {
	authProvider := auth.NewIAPIProvider(rt, internalAPIHost)
	memCache := cache.NewMemoryCache()
	return middleware.Chain(
		ip.Middleware,
		sdkrouter.Middleware(rt),
		auth.Middleware(authProvider),
		tracker.Middleware(boil.GetDB()),
		cache.Middleware(memCache),
	)
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
