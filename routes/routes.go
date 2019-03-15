package routes

import (
	raven "github.com/getsentry/raven-go"
	"github.com/gorilla/mux"
)

// InstallRoutes sets up global API handlers
func InstallRoutes(r *mux.Router) {
	rh := raven.RecoveryHandler
	r.HandleFunc("/", Index)
	r.HandleFunc("/api/proxy", rh(Proxy))
	r.HandleFunc("/api/proxy/", rh(Proxy))
	r.HandleFunc("/content/claims/{uri}/{claim}/{filename}", rh(ContentByClaimsURI))
	r.HandleFunc("/content/url", rh(ContentByURL))
}
