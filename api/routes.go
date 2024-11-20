package api

import (
	"net/http"
	"net/http/pprof"
	"strings"
	"sync"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/asynquery"
	"github.com/OdyseeTeam/odysee-api/app/auth"
	"github.com/OdyseeTeam/odysee-api/app/geopublish"
	gpmetrics "github.com/OdyseeTeam/odysee-api/app/geopublish/metrics"
	"github.com/OdyseeTeam/odysee-api/app/proxy"
	"github.com/OdyseeTeam/odysee-api/app/publish"
	"github.com/OdyseeTeam/odysee-api/app/query"
	"github.com/OdyseeTeam/odysee-api/app/sdkrouter"
	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/ip"
	"github.com/OdyseeTeam/odysee-api/internal/metrics"
	"github.com/OdyseeTeam/odysee-api/internal/middleware"
	"github.com/OdyseeTeam/odysee-api/internal/monitor"
	"github.com/OdyseeTeam/odysee-api/internal/status"
	"github.com/OdyseeTeam/odysee-api/internal/storage"
	"github.com/OdyseeTeam/odysee-api/pkg/keybox"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"
	"github.com/OdyseeTeam/odysee-api/pkg/redislocker"
	"github.com/OdyseeTeam/odysee-api/pkg/sturdycache"
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

type RoutesOptions struct {
	EnableV3Publish bool
	EnableProfiling bool
}

// emptyHandler can be used when you just need to let middlewares do their job and no actual response is needed.
func emptyHandler(_ http.ResponseWriter, _ *http.Request) {}

// InstallRoutes sets up global API handlers
func InstallRoutes(r *mux.Router, sdkRouter *sdkrouter.Router, opts *RoutesOptions) {
	if opts == nil {
		opts = &RoutesOptions{}
	}
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

	v1Router.HandleFunc("/paid/pubkey", paid.HandlePublicKeyRequest).Methods(http.MethodGet)

	internalRouter := r.PathPrefix("/internal").Subrouter()
	internalRouter.Handle("/metrics", promhttp.Handler())

	if opts.EnableProfiling {
		pr := internalRouter.PathPrefix("/pprof").Subrouter()
		pr.HandleFunc("/symbol", pprof.Symbol).Methods(http.MethodPost)
		pr.HandleFunc("/", pprof.Index)
		pr.HandleFunc("/cmdline", pprof.Cmdline)
		pr.HandleFunc("/profile", pprof.Profile)
		pr.HandleFunc("/symbol", pprof.Symbol)
		pr.HandleFunc("/trace", pprof.Trace)
		pr.Handle("/allocs", pprof.Handler("allocs"))
		pr.Handle("/block", pprof.Handler("block"))
		pr.Handle("/goroutine", pprof.Handler("goroutine"))
		pr.Handle("/heap", pprof.Handler("heap"))
		pr.Handle("/mutex", pprof.Handler("mutex"))
		pr.Handle("/threadcreate", pprof.Handler("threadcreate"))
	}

	v2Router := r.PathPrefix("/api/v2").Subrouter()
	v2Router.Use(defaultMiddlewares(oauthAuther, legacyProvider, sdkRouter))
	status.InstallRoutes(v2Router)

	composer := tusd.NewStoreComposer()
	store := filestore.New(uploadPath)
	store.UseIn(composer)

	redisOpts, err := config.GetRedisLockerOpts()
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

	tusHandler, err := publish.NewTusHandler(
		publish.WithAuther(oauthAuther),
		publish.WithLegacyProvider(legacyProvider),
		publish.WithTusConfig(tusCfg),
		publish.WithUploadPath(uploadPath),
	)
	if err != nil {
		logger.Log().WithError(err).Fatal(err)
	}

	tusRouter := v2Router.PathPrefix("/publish").Subrouter()
	tusRouter.Use(tusHandler.Middleware)
	tusRouter.HandleFunc("/", tusHandler.PostFile).Methods(http.MethodPost).Name("tus_publish")
	tusRouter.HandleFunc("/{id}", tusHandler.HeadFile).Methods(http.MethodHead)
	tusRouter.HandleFunc("/{id}", tusHandler.PatchFile).Methods(http.MethodPatch)
	tusRouter.HandleFunc("/{id}", tusHandler.DelFile).Methods(http.MethodDelete)
	tusRouter.HandleFunc("/{id}/notify", tusHandler.Notify).Methods(http.MethodPost)
	tusRouter.PathPrefix("/").HandlerFunc(emptyHandler).Methods(http.MethodOptions)

	var v3Handler *geopublish.Handler
	gpl := zapadapter.NewNamedKV("geopublish", config.GetLoggingOpts())
	if opts.EnableV3Publish {
		v3Router := r.PathPrefix("/api/v3").Subrouter()
		v3Router.Use(defaultMiddlewares(oauthAuther, legacyProvider, sdkRouter))
		ug := auth.NewUniversalUserGetter(oauthAuther, legacyProvider, gpl)
		gPath := config.GetGeoPublishSourceDir()
		v3Handler, err = geopublish.InstallRoutes(v3Router.PathPrefix("/publish").Subrouter(), ug, gPath, "/api/v3/publish/", gpl)
		if err != nil {
			panic(err)
		}
	}

	keyfob, err := keybox.KeyfobFromString(config.GetUploadTokenPrivateKey())
	if err != nil {
		panic(err)
	}

	asynqueryBusOpts, err := config.GetAsynqueryRequestsConnOpts()
	if err != nil {
		panic(err)
	}
	launcher := asynquery.NewLauncher(
		asynquery.WithRequestsConnOpts(asynqueryBusOpts),
		asynquery.WithLogger(zapadapter.NewKV(nil)),
		asynquery.WithPrivateKey(keyfob.PrivateKey()),
		asynquery.WithDB(storage.DB),
		asynquery.WithUploadServiceURL(config.GetUploadServiceURL()),
	)

	err = launcher.InstallRoutes(v1Router)
	if err != nil {
		panic(err)
	}

	go launcher.Start()

	onceMetrics.Do(func() {
		gpmetrics.RegisterMetrics(nil)
		redislocker.RegisterMetrics(nil)
		if !opts.EnableV3Publish {
			tus2metrics := prometheuscollector.New(tusHandler.Metrics)
			prometheus.MustRegister(tus2metrics)
		} else {
			tus3metrics := prometheuscollector.New(v3Handler.Metrics)
			prometheus.MustRegister(tus3metrics)
		}
	})
}

func defaultMiddlewares(oauthAuther auth.Authenticator, legacyProvider auth.Provider, router *sdkrouter.Router) mux.MiddlewareFunc {
	store, err := sturdycache.NewReplicatedCache(
		config.GetSturdyCacheMaster(),
		config.GetSturdyCacheReplicas(),
		config.GetSturdyCachePassword(),
	)
	if err != nil {
		panic(err)
	}
	cache := query.NewQueryCache(store)
	logger.Log().Infof("cache configured: master=%s, replicas=%s", config.GetSturdyCacheMaster(), config.GetSturdyCacheReplicas())

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
		query.CacheMiddleware(cache),
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
