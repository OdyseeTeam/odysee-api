package api

import (
	"net/http/pprof"

	"github.com/lbryio/lbrytv/app/player"
	"github.com/lbryio/lbrytv/app/proxy"
	"github.com/lbryio/lbrytv/app/publish"
	"github.com/lbryio/lbrytv/app/users"

	"github.com/gorilla/mux"
)

// InstallRoutes sets up global API handlers
func InstallRoutes(proxyService *proxy.ProxyService, r *mux.Router) {
	authenticator := users.NewAuthenticator(users.NewWalletService())
	proxyHandler := proxy.NewRequestHandler(proxyService)
	upHandler, err := publish.NewUploadHandler(publish.UploadOpts{ProxyService: proxyService})
	if err != nil {
		panic(err)
	}

	r.HandleFunc("/", Index)

	v1Router := r.PathPrefix("/api/v1").Subrouter()
	v1Router.HandleFunc("/proxy", proxyHandler.HandleOptions).Methods("OPTIONS")
	v1Router.HandleFunc("/proxy", authenticator.Wrap(upHandler.Handle)).MatcherFunc(upHandler.CanHandle)
	v1Router.HandleFunc("/proxy", proxyHandler.Handle)

	player.InstallRoutes(r)

	debugRouter := r.PathPrefix("/superdebug/pprof").Subrouter()
	debugRouter.HandleFunc("/", pprof.Index)
	debugRouter.HandleFunc("/cmdline", pprof.Cmdline)
	debugRouter.HandleFunc("/profile", pprof.Profile)
	debugRouter.HandleFunc("/symbol", pprof.Symbol)
	debugRouter.HandleFunc("/trace", pprof.Trace)
	debugRouter.Handle("/heap", pprof.Handler("heap"))
}
