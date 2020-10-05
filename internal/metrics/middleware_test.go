package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/internal/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type FakeObserver struct {
	value *float64
}

func (o FakeObserver) Observe(v float64) {
	*(o.value) += v
}

func TestMeasure(t *testing.T) {
	v := float64(0)
	o := FakeObserver{&v}
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		time.Sleep(10 * time.Millisecond)
		AddObserver(r, o)
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
