package api

import (
	"github.com/lbryio/lbrytv/app/proxy"
	"github.com/lbryio/lbrytv/app/publish"
	"github.com/lbryio/lbrytv/app/users"
	"github.com/lbryio/lbrytv/config"

	"github.com/gorilla/mux"
)

// InstallRoutes sets up global API handlers
func InstallRoutes(ps *proxy.Service, r *mux.Router) {
	r.HandleFunc("/", Index)

	proxyServer := proxy.NewRequestServer(ps)

	r.HandleFunc("/api/proxy", captureErrors(proxyServer.Handle))
	r.HandleFunc("/api/proxy/", captureErrors(proxyServer.Handle))
	r.HandleFunc("/content/claims/{uri}/{claim}/{filename}", captureErrors(ContentByClaimsURI))
	r.HandleFunc("/content/url", captureErrors(ContentByURL))

	actionsRouter := r.Path("/api/v1/actions").Subrouter()
	authenticator := users.NewAuthenticator(users.NewUserService())
	lbrynetPublisher := &publish.LbrynetPublisher{}
	UploadHandler := publish.NewUploadHandler(config.GetPublishDir(), lbrynetPublisher)
	actionsRouter.HandleFunc("/publish", authenticator.Wrap(UploadHandler.Handle)).Headers(users.TokenHeader, "")
}
