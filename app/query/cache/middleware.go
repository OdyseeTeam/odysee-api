package cache

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
)

const ContextKey = "cache"

func IsOnRequest(r *http.Request) bool {
	return r.Context().Value(ContextKey) != nil
}

func FromRequest(r *http.Request) QueryCache {
	v := r.Context().Value(ContextKey)
	if v == nil {
		panic("cache.Middleware is required")
	}
	return v.(QueryCache)
}

func AddToRequest(c QueryCache, fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r.Clone(context.WithValue(r.Context(), ContextKey, c)))
	}
}

func Middleware(c QueryCache) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return AddToRequest(c, next.ServeHTTP)
	}
}
