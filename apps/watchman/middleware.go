package watchman

import (
	"context"
	"net"
	"net/http"
)

type ctxKey int

const RemoteAddressKey ctxKey = iota + 1

func RemoteAddressMiddleware() func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			req := r
			ctx := context.WithValue(r.Context(), RemoteAddressKey, from(r))
			req = r.WithContext(ctx)
			h.ServeHTTP(w, req)
		})
	}
}

// from makes a best effort to compute the request client IP.
func from(req *http.Request) string {
	if f := req.Header.Get("X-Forwarded-For"); f != "" {
		return f
	}
	f := req.RemoteAddr
	ip, _, err := net.SplitHostPort(f)
	if err != nil {
		return f
	}
	return ip
}
