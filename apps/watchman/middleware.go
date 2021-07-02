package watchman

import (
	"context"
	"net/http"

	"github.com/lbryio/lbrytv/internal/ip"
)

type ctxKey int

const RemoteAddressKey ctxKey = iota + 1

func RemoteAddressMiddleware() func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		// A HTTP handler is a function.
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			req := r
			ctx := context.WithValue(r.Context(), RemoteAddressKey, ip.AddressForRequest(r))
			req = r.WithContext(ctx)
			// Call initial handler.
			h.ServeHTTP(w, req)
		})
	}
}
