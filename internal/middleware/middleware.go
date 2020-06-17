package middleware

import (
	"net/http"

	"github.com/gorilla/mux"
)

// Chain chains multiple middleware together
func Chain(mws ...mux.MiddlewareFunc) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next) // apply in reverse to get the intuitive LIFO order
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}

// Apply applies middlewares to HandlerFunc
func Apply(mw mux.MiddlewareFunc, handler http.HandlerFunc) http.Handler {
	return mw(handler)
}
