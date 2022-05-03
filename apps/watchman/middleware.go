package watchman

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"time"

	hmw "goa.design/goa/v3/http/middleware"
)

type ctxKey int

const RemoteAddressKey ctxKey = iota + 1

// ObserveResponse returns a middleware that observes HTTP request processing times and response codes.
func ObserveResponse() func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			rw := hmw.CaptureResponse(w)
			h.ServeHTTP(rw, r)
			httpResponses.WithLabelValues("playback", strconv.Itoa(rw.StatusCode)).Observe(time.Since(started).Seconds())
		})
	}
}
func RemoteAddressMiddleware() func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), RemoteAddressKey, from(r))
			h.ServeHTTP(w, r.WithContext(ctx))
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
