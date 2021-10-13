package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/lbryio/lbrytv-player/pkg/paid"
	"github.com/lbryio/lbrytv/app/auth"
	"github.com/lbryio/lbrytv/app/proxy"
	"github.com/lbryio/lbrytv/app/publish"
	"github.com/lbryio/lbrytv/app/query/cache"
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/apps/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/ip"
	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/middleware"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/internal/status"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	"github.com/tus/tusd/pkg/filestore"
	tusd "github.com/tus/tusd/pkg/handler"
)

var logger = monitor.NewModuleLogger("api")

// emptyHandler can be used when you just need to let middlewares do their job and no actual response is needed.
func emptyHandler(_ http.ResponseWriter, _ *http.Request) {}

// InstallRoutes sets up global API handlers
func InstallRoutes(r *mux.Router, sdkRouter *sdkrouter.Router) {
	uploadPath := config.GetPublishSourceDir()
	authProvider := auth.NewIAPIProvider(sdkRouter, config.GetInternalAPIHost())

	upHandler := &publish.Handler{UploadPath: uploadPath}
	r.Use(methodTimer)

	r.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("lbrytv api"))
	})
	r.HandleFunc("", emptyHandler)

	v1Router := r.PathPrefix("/api/v1").Subrouter()
	v1Router.Use(defaultMiddlewares(sdkRouter, authProvider))

	v1Router.HandleFunc("/proxy", upHandler.Handle).MatcherFunc(publish.CanHandle)
	v1Router.HandleFunc("/proxy", proxy.Handle).Methods(http.MethodPost)
	v1Router.HandleFunc("/proxy", emptyHandler).Methods(http.MethodOptions)

	v1Router.HandleFunc("/metric/ui", metrics.TrackUIMetric).Methods(http.MethodPost)
	v1Router.HandleFunc("/metric/ui", emptyHandler).Methods(http.MethodOptions)

	v1Router.HandleFunc("/status", status.GetStatus).Methods(http.MethodGet)
	v1Router.HandleFunc("/paid/pubkey", paid.HandlePublicKeyRequest).Methods(http.MethodGet)

	internalRouter := r.PathPrefix("/internal").Subrouter()
	internalRouter.Handle("/metrics", promhttp.Handler())

	v2Router := r.PathPrefix("/api/v2").Subrouter()
	v2Router.Use(defaultMiddlewares(sdkRouter, authProvider))
	v2Router.HandleFunc("/status", status.GetStatusV2).Methods(http.MethodGet)
	v2Router.HandleFunc("/status", emptyHandler).Methods(http.MethodOptions)

	composer := tusd.NewStoreComposer()
	store := filestore.FileStore{
		Path: uploadPath,
	}
	store.UseIn(composer)
	tusCfg := tusd.Config{
		BasePath:      "/api/v2/publish/",
		StoreComposer: composer,
	}

	tusHandler, err := publish.NewTusHandler(authProvider, tusCfg, uploadPath)
	if err != nil {
		logger.Log().WithError(err).Fatal(err)
	}

	tusRouter := v2Router.PathPrefix("/publish").Subrouter()
	tusRouter.Use(tusHandler.Middleware)
	tusRouter.HandleFunc("/", tusHandler.PostFile).Methods(http.MethodPost)
	tusRouter.HandleFunc("/{id}", tusHandler.HeadFile).Methods(http.MethodHead)
	tusRouter.HandleFunc("/{id}", tusHandler.PatchFile).Methods(http.MethodPatch)
	tusRouter.HandleFunc("/{id}/notify", tusHandler.Notify).Methods(http.MethodPost)
	tusRouter.PathPrefix("/").HandlerFunc(emptyHandler).Methods(http.MethodOptions)
}

func defaultMiddlewares(rt *sdkrouter.Router, authProvider auth.Provider) mux.MiddlewareFunc {
	memCache := cache.NewMemoryCache()
	defaultHeaders := []string{
		wallet.TokenHeader, "X-Requested-With", "Content-Type", "Accept",
	}
	c := cors.New(cors.Options{
		AllowedOrigins:   config.GetCORSDomains(),
		AllowCredentials: true,
		AllowedHeaders:   append(defaultHeaders, publish.TusHeaders...),
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodHead},
		MaxAge:           600,
	})
	logger.Log().Infof("added CORS domains: %v", config.GetCORSDomains())

	return middleware.Chain(
		metrics.MeasureMiddleware(),
		c.Handler,
		ip.Middleware,
		sdkrouter.Middleware(rt),
		auth.Middleware(authProvider),
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
