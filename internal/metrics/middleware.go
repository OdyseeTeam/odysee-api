package metrics

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/lbryio/lbrytv/internal/errors"
	"github.com/prometheus/client_golang/prometheus"
)

type key int

const timerContextKey key = iota

// Measure middleware starts a timer whenever a request is performed.
// It should be added as first in the chain of middlewares.
// Note that it doesn't catch any metrics by itself,
// HTTP handlers are expected to add their own by calling AddObserver.
func Measure() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t := StartTimer()
			Logger.Log().Debugf("timer %p started", t)
			rc := r.Clone(context.WithValue(r.Context(), timerContextKey, t))
			next.ServeHTTP(w, rc)
			t.Stop()
			Logger.Log().Debugf("timer %p stopped (%.6fs)", t, t.Duration)
		})
	}
}

// AddObserver adds Prometheus metric to a chain of observers for a given HTTP request.
func AddObserver(r *http.Request, o prometheus.Observer) error {
	v := r.Context().Value(timerContextKey)
	if v == nil {
		return errors.Err("metrics.Measure middleware is required")
	}
	t := v.(*Timer)
	t.AddObserver(o)
	return nil
}

// GetDuration returns current duration of the request in seconds.
// Returns a negative value when Measure middleware is not present.
func GetDuration(r *http.Request) float64 {
	v := r.Context().Value(timerContextKey)
	if v == nil {
		return -1
	}
	t := v.(*Timer)
	return t.GetDuration()
}
