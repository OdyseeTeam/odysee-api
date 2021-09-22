package ip

import (
	"context"
	"net/http"
)

type ctxKey int

const contextKey ctxKey = iota

// FromRequest retrieves remote user IP from http.Request that went through our middleware
func FromRequest(r *http.Request) string {
	v := r.Context().Value(contextKey)
	if v == nil {
		logger.Log().Warn("ip.FromRequest was called but ip.Middleware wasn't applied")
		return ""
	}
	return v.(string)
}

// Middleware will attach remote user IP to every request
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteIP := AddressForRequest(r.Header, r.RemoteAddr)
		next.ServeHTTP(w, r.Clone(context.WithValue(r.Context(), contextKey, remoteIP)))
	})
}
