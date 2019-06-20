package routes

import (
	"net/http"

	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/gorilla/mux"
)

func captureErrors(handler func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	sentryHandler := sentryhttp.New(sentryhttp.Options{})
	return sentryHandler.HandleFunc(handler)
}

// InstallRoutes sets up global API handlers
func InstallRoutes(r *mux.Router) {
	r.HandleFunc("/", Index)
	r.HandleFunc("/api/proxy", captureErrors(Proxy))
	r.HandleFunc("/api/proxy/", captureErrors(Proxy))
	r.HandleFunc("/content/claims/{uri}/{claim}/{filename}", captureErrors(ContentByClaimsURI))
	r.HandleFunc("/content/url", captureErrors(ContentByURL))
}
