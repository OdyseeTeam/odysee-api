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

// Chain chains multiple middleware together
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

func AddObserver(r *http.Request, o prometheus.Observer) error {
	v := r.Context().Value(timerContextKey)
	if v == nil {
		return errors.Err("metrics.Measure middleware is required")
	}
	t := v.(*Timer)
	t.AddObserver(o)
	return nil
}
