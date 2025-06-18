package ip

import (
	"context"
	"net/http"
)

type ctxKey int

const remoteIPContextKey ctxKey = iota

// FromRequest retrieves remote user IP from http.Request that went through our middleware
func FromRequest(r *http.Request) string {
	v := r.Context().Value(remoteIPContextKey)
	if v == nil {
		return ""
	}
	return v.(string)
}

// Middleware will attach remote user IP to every request
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteIP := ForRequest(r)
		next.ServeHTTP(w, r.Clone(context.WithValue(r.Context(), remoteIPContextKey, remoteIP)))
	})
}
