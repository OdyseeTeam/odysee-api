package api

import (
	"net/http"

	"github.com/lbryio/lbrytv/app/proxy"

	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/gorilla/mux"
)

func captureErrors(handler func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	sentryHandler := sentryhttp.New(sentryhttp.Options{})
	return sentryHandler.HandleFunc(handler)
}

// InstallRoutes sets up global API handlers
func InstallRoutes(ps *proxy.Service, r *mux.Router) {
	r.HandleFunc("/", Index)

	proxyHandler := &proxy.RequestHandler{Service: ps}
	r.HandleFunc("/api/proxy", captureErrors(proxyHandler.Handle))
	r.HandleFunc("/api/proxy/", captureErrors(proxyHandler.Handle))
	r.HandleFunc("/content/claims/{uri}/{claim}/{filename}", captureErrors(ContentByClaimsURI))
	r.HandleFunc("/content/url", captureErrors(ContentByURL))
}
