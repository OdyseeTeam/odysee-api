package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/internal/middleware"

	"github.com/gorilla/mux"
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

func TestMeasureMiddleware(t *testing.T) {
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
		middleware.Apply(MeasureMiddleware(), handler).ServeHTTP(rr, r)
	})

	res := rr.Result()
	assert.Equal(t, http.StatusForbidden, res.StatusCode)
	assert.Greater(t, *o.value, float64(0.001))
}

func TestMeasureMiddlewareExtraMiddleware(t *testing.T) {
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
		middleware.Apply(middleware.Chain(MeasureMiddleware(), waitingMiddleware()), handler).ServeHTTP(rr, r)
	})

	res := rr.Result()
	assert.Equal(t, http.StatusForbidden, res.StatusCode)
	assert.Greater(t, *o.value, float64(0.015))
}

func TestMeasureMiddlewarePanic(t *testing.T) {
	v := float64(0)
	o := fakeObserver{&v}
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		time.Sleep(10 * time.Millisecond)
		err := AddObserver(r, o)
		require.NoError(t, err)
		panic("oh")
	}

	r, err := http.NewRequest("POST", "/", nil)
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	assert.PanicsWithValue(t, "oh", func() {
		middleware.Apply(MeasureMiddleware(), handler).ServeHTTP(rr, r)
	})

	res := rr.Result()
	assert.Equal(t, http.StatusForbidden, res.StatusCode)
	assert.Greater(t, *o.value, float64(0.005))
}

func TestAddObserverMissingMiddleware(t *testing.T) {
	v := float64(0)
	o := fakeObserver{&v}
	handler := func(w http.ResponseWriter, r *http.Request) {
		err := AddObserver(r, o)
		require.EqualError(t, err, "metrics.MeasureMiddleware middleware is required")
	}

	r, err := http.NewRequest("POST", "/", nil)
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	assert.NotPanics(t, func() {
		middleware.Apply(waitingMiddleware(), handler).ServeHTTP(rr, r)
	})
}

func TestGetDuration(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		d := GetDuration(r)
		assert.Greater(t, d, float64(0.005))
	}

	r, err := http.NewRequest("POST", "/", nil)
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	assert.NotPanics(t, func() {
		middleware.Apply(middleware.Chain(MeasureMiddleware(), waitingMiddleware()), handler).ServeHTTP(rr, r)
	})
}

func TestGetDurationNoMiddleware(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		d := GetDuration(r)
		assert.Less(t, d, float64(0))
	}

	r, err := http.NewRequest("POST", "/", nil)
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	assert.NotPanics(t, func() {
		middleware.Apply(middleware.Chain(waitingMiddleware()), handler).ServeHTTP(rr, r)
	})
}
