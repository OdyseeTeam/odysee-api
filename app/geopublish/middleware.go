package geopublish

import (
	"net/http"
	"regexp"

	"github.com/getsentry/sentry-go"
	"github.com/gorilla/mux"
)

var reExtractFileID = regexp.MustCompile(`([^/]+)\/?$`)

func TracingMiddleware() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
				uploadID := extractID(r)
				if uploadID != "" {
					hub.Scope().AddBreadcrumb(&sentry.Breadcrumb{
						Category: "upload",
						Message:  "ID",
						Level:    sentry.LevelInfo,
					}, 999)
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func extractID(r *http.Request) string {
	params := mux.Vars(r)
	return params["id"]
}
