package api

import (
	"net/http"

	sentryhttp "github.com/getsentry/sentry-go/http"
)

func captureErrors(next http.HandlerFunc) http.HandlerFunc {
	sentryHandler := sentryhttp.New(sentryhttp.Options{})
	return sentryHandler.HandleFunc(next)
}
