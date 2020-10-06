package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/lbryio/lbrytv/internal/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeObserver struct {
	value *float64
}

func (o fakeObserver) Observe(v float64) {
	*(o.value) += v
}

func waitingMiddleware() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond)
			next.ServeHTTP(w, r)
		})
	}
}

func TestMeasure(t *testing.T) {
	v := float64(0)
	o := fakeObserver{&v}
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		time.Sleep(10 * time.Millisecond)
		err := AddObserver(r, o)
		require.NoError(t, err)
	}

	r, err := http.NewRequest("POST", "/", nil)
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	assert.NotPanics(t, func() {
		middleware.Apply(Measure(), handler).ServeHTTP(rr, r)
	})

	res := rr.Result()
	assert.Equal(t, http.StatusForbidden, res.StatusCode)
	assert.Greater(t, *o.value, float64(0.001))
}

func TestMeasureExtraMiddleware(t *testing.T) {
	v := float64(0)
	o := fakeObserver{&v}
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		time.Sleep(10 * time.Millisecond)
		err := AddObserver(r, o)
		require.NoError(t, err)
	}

	r, err := http.NewRequest("POST", "/", nil)
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	assert.NotPanics(t, func() {
		middleware.Apply(middleware.Chain(Measure(), waitingMiddleware()), handler).ServeHTTP(rr, r)
	})

	res := rr.Result()
	assert.Equal(t, http.StatusForbidden, res.StatusCode)
	assert.Greater(t, *o.value, float64(0.015))
}

func TestAddObserverMissingMiddleware(t *testing.T) {
	v := float64(0)
	o := fakeObserver{&v}
	handler := func(w http.ResponseWriter, r *http.Request) {
		err := AddObserver(r, o)
		require.EqualError(t, err, "metrics.Measure middleware is required")
	}

	r, err := http.NewRequest("POST", "/", nil)
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	assert.NotPanics(t, func() {
		middleware.Apply(waitingMiddleware(), handler).ServeHTTP(rr, r)
	})
}
