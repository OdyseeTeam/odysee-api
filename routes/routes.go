package routes

import (
	"net/http"

	raven "github.com/getsentry/raven-go"
	"github.com/gorilla/mux"
	"github.com/lbryio/lbrytv/users"
)

// CaptureErrors wraps http handler with raven.RecoveryHandler which captures unhandled exceptions to sentry.io
func CaptureErrors(handler func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return raven.RecoveryHandler(handler)
}

// InstallRoutes sets up global API handlers
func InstallRoutes(r *mux.Router) {
	r.HandleFunc("/", Index)
	r.HandleFunc("/api/proxy", CaptureErrors(Proxy))
	r.HandleFunc("/api/proxy/", CaptureErrors(Proxy))
	r.HandleFunc("/content/claims/{uri}/{claim}/{filename}", CaptureErrors(ContentByClaimsURI))
	r.HandleFunc("/content/url", CaptureErrors(ContentByURL))

	r.HandleFunc("/api/user/{method}", CaptureErrors(users.HandleMethod))
}
