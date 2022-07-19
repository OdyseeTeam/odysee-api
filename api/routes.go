package api

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/auth"
	"github.com/OdyseeTeam/odysee-api/app/proxy"
	"github.com/OdyseeTeam/odysee-api/app/publish"
	"github.com/OdyseeTeam/odysee-api/app/query/cache"
	"github.com/OdyseeTeam/odysee-api/app/sdkrouter"
	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/ip"
	"github.com/OdyseeTeam/odysee-api/internal/metrics"
	"github.com/OdyseeTeam/odysee-api/internal/middleware"
	"github.com/OdyseeTeam/odysee-api/internal/monitor"
	"github.com/OdyseeTeam/odysee-api/internal/status"
	"github.com/OdyseeTeam/odysee-api/pkg/redislocker"
	"github.com/OdyseeTeam/player-server/pkg/paid"

	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	"github.com/tus/tusd/pkg/filestore"
	tusd "github.com/tus/tusd/pkg/handler"
	"github.com/tus/tusd/pkg/prometheuscollector"
)

const preflightDuration = 86400

var logger = monitor.NewModuleLogger("api")

var onceMetrics sync.Once

// emptyHandler can be used when you just need to let middlewares do their job and no actual response is needed.
func emptyHandler(_ http.ResponseWriter, _ *http.Request) {}

// InstallRoutes sets up global API handlers
func InstallRoutes(r *mux.Router, sdkRouter *sdkrouter.Router) {
	uploadPath := config.GetPublishSourceDir()

	upHandler := &publish.Handler{UploadPath: uploadPath}
	oauthAuther, err := wallet.NewOauthAuthenticator(
		config.GetOauthProviderURL(), config.GetOauthClientID(), config.GetInternalAPIHost(), sdkRouter)
	if err != nil {
		panic(err)
	}
	legacyProvider := auth.NewIAPIProvider(sdkRouter, config.GetInternalAPIHost())
	sentryHandler := sentryhttp.New(sentryhttp.Options{})

	r.Use(methodTimer, sentryHandler.Handle)

	r.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("odysee api"))
	})
	r.HandleFunc("", emptyHandler)

	v1Router := r.PathPrefix("/api/v1").Subrouter()
	v1Router.Use(defaultMiddlewares(oauthAuther, legacyProvider, sdkRouter))

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
	v2Router.Use(defaultMiddlewares(oauthAuther, legacyProvider, sdkRouter))
	v2Router.HandleFunc("/status", status.GetStatusV2).Methods(http.MethodGet)
	v2Router.HandleFunc("/status", emptyHandler).Methods(http.MethodOptions)

	composer := tusd.NewStoreComposer()
	store := filestore.New(uploadPath)
	store.UseIn(composer)

	redisOpts, err := config.GetRedisOpts()
	if err != nil {
		panic(err)
	}
	locker, err := redislocker.New(redisOpts)
	if err != nil {
		logger.Log().WithError(err).Fatal("cannot start redislocker")
	}
	locker.UseIn(composer)

	tusCfg := tusd.Config{
		BasePath:      "/api/v2/publish/",
		StoreComposer: composer,
	}

	tusHandler, err := publish.NewTusHandler(oauthAuther, legacyProvider, tusCfg, uploadPath)
	if err != nil {
		logger.Log().WithError(err).Fatal(err)
	}

	onceMetrics.Do(func() {
		redislocker.RegisterMetrics()
		collector := prometheuscollector.New(tusHandler.Metrics)
		prometheus.MustRegister(collector)
	})

	tusRouter := v2Router.PathPrefix("/publish").Subrouter()
	tusRouter.Use(tusHandler.Middleware)
	tusRouter.HandleFunc("/", tusHandler.PostFile).Methods(http.MethodPost)
	tusRouter.HandleFunc("/{id}", tusHandler.HeadFile).Methods(http.MethodHead)
	tusRouter.HandleFunc("/{id}", tusHandler.PatchFile).Methods(http.MethodPatch)
	tusRouter.HandleFunc("/{id}", tusHandler.DelFile).Methods(http.MethodDelete)
	tusRouter.HandleFunc("/{id}/notify", tusHandler.Notify).Methods(http.MethodPost)
	tusRouter.PathPrefix("/").HandlerFunc(emptyHandler).Methods(http.MethodOptions)
}

func defaultMiddlewares(oauthAuther auth.Authenticator, legacyProvider auth.Provider, router *sdkrouter.Router) mux.MiddlewareFunc {
	queryCache, err := cache.New(cache.DefaultConfig())
	if err != nil {
		panic(err)
	}
	defaultHeaders := []string{
		wallet.LegacyTokenHeader, wallet.AuthorizationHeader, "X-Requested-With", "Content-Type", "Accept",
	}
	c := cors.New(cors.Options{
		AllowedOrigins:   config.GetCORSDomains(),
		AllowCredentials: true,
		AllowedHeaders:   append(defaultHeaders, publish.TusHeaders...),
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodHead, http.MethodDelete},
		MaxAge:           preflightDuration,
	})
	logger.Log().Infof("added CORS domains: %v", config.GetCORSDomains())

	return middleware.Chain(
		metrics.MeasureMiddleware(),
		c.Handler,
		ip.Middleware,
		sdkrouter.Middleware(router),
		auth.Middleware(oauthAuther), // Will pass forward user/error to next
		auth.LegacyMiddleware(legacyProvider),
		cache.Middleware(queryCache),
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
