package query

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
)

type cacheKey struct{}

func HasCache(r *http.Request) bool {
	return r.Context().Value(cacheKey{}) != nil
}

func CacheFromRequest(r *http.Request) *QueryCache {
	v := r.Context().Value(cacheKey{})
	if v == nil {
		panic("query.CacheMiddleware is required")
	}
	return v.(*QueryCache)
}

func AddCacheToRequest(cache *QueryCache, fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r.Clone(context.WithValue(r.Context(), cacheKey{}, cache)))
	}
}

func CacheMiddleware(cache *QueryCache) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return AddCacheToRequest(cache, next.ServeHTTP)
	}
}
