package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/lbryio/lbrytv/app/proxy"
	"github.com/lbryio/lbrytv/app/publish"
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/app/users"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/status"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// InstallRoutes sets up global API handlers
func InstallRoutes(proxyService *proxy.Service, r *mux.Router) {
	authenticator := users.NewAuthenticator(users.NewWalletService(proxyService.SDKRouter))
	proxyHandler := proxy.NewRequestHandler(proxyService)
	upHandler, err := publish.NewUploadHandler(publish.UploadOpts{ProxyService: proxyService})
	if err != nil {
		panic(err)
	}

	r.Use(methodTimer)

	r.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, config.GetProjectURL(), http.StatusSeeOther)
	})

	v1Router := r.PathPrefix("/api/v1").Subrouter()
	v1Router.HandleFunc("/proxy", proxy.HandleCORS).Methods(http.MethodOptions)
	v1Router.HandleFunc("/proxy", authenticator.Wrap(upHandler.Handle)).MatcherFunc(upHandler.CanHandle)
	v1Router.HandleFunc("/proxy", proxyHandler.Handle)
	v1Router.HandleFunc("/metric/ui", metrics.TrackUIMetric).Methods(http.MethodPost)

	internalRouter := r.PathPrefix("/internal").Subrouter()
	internalRouter.Handle("/metrics", promhttp.Handler())
	internalRouter.HandleFunc("/status", injectSDKRouter(proxyService.SDKRouter, status.GetStatus))
	internalRouter.HandleFunc("/whoami", status.WhoAMI)
}

// i can't tell if this is really a best practice or a hack
func injectSDKRouter(rt *sdkrouter.Router, fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r.Clone(context.WithValue(r.Context(), status.SDKRouterContextKey, rt)))
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
