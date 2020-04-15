package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/lbryio/lbrytv/app/auth"
	"github.com/lbryio/lbrytv/app/proxy"
	"github.com/lbryio/lbrytv/app/publish"
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/internal/responses"
	"github.com/lbryio/lbrytv/internal/status"
	"github.com/ybbus/jsonrpc"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var logger = monitor.NewModuleLogger("api")

// InstallRoutes sets up global API handlers
func InstallRoutes(r *mux.Router, sdkRouter *sdkrouter.Router) {
	upHandler := &publish.Handler{UploadPath: config.GetPublishSourceDir()}

	r.Use(recoveryHandler)
	r.Use(methodTimer)

	r.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, config.GetProjectURL(), http.StatusSeeOther)
	})

	v1Router := r.PathPrefix("/api/v1").Subrouter()
	v1Router.Use(sdkrouter.Middleware(sdkRouter))
	authProvider := auth.WalletAndInternalAPIProvider(sdkRouter, config.GetInternalAPIHost())
	v1Router.Use(auth.Middleware(authProvider))
	v1Router.HandleFunc("/proxy", proxy.HandleCORS).Methods(http.MethodOptions)
	v1Router.HandleFunc("/proxy", upHandler.Handle).MatcherFunc(upHandler.CanHandle)
	v1Router.HandleFunc("/proxy", proxy.Handle)
	v1Router.HandleFunc("/metric/ui", metrics.TrackUIMetric).Methods(http.MethodPost)

	internalRouter := r.PathPrefix("/internal").Subrouter()
	internalRouter.Handle("/metrics", promhttp.Handler())
	internalRouter.HandleFunc("/status", sdkrouter.AddToRequest(sdkRouter, status.GetStatus))
	internalRouter.HandleFunc("/whoami", status.WhoAMI)
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

func recoveryHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recovered, stack := func() (err error, stack []byte) {
			defer func() {
				if r := recover(); r != nil {
					var ok bool
					err, ok = r.(error)
					if !ok {
						err = fmt.Errorf("%v", r)
					}
					if !config.IsProduction() {
						stack = debug.Stack()
					}
				}
			}()
			next.ServeHTTP(w, r)
			return err, nil
		}()
		if recovered != nil {
			logger.Log().Errorf("PANIC %v, trace %s", recovered, stack)
			responses.AddJSONContentType(w)
			rsp, _ := json.Marshal(jsonrpc.RPCResponse{
				JSONRPC: "2.0",
				Error: &jsonrpc.RPCError{
					Code:    -1,
					Message: recovered.Error(),
					Data:    stack,
				},
			})
			w.Write(rsp)
		}
	})
}
