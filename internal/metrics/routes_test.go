package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lbryio/lbrytv/api"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBufferEvent(t *testing.T) {
	name := "buffer"
	rr := testMetricUIEvent(t, http.MethodPost, name, "")
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, `{"name":"`+name+`"}`, rr.Body.String())
}

func TestInvalidEvent(t *testing.T) {
	name := "win-the-lottery"
	rr := testMetricUIEvent(t, http.MethodPost, name, "")
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, `{"error":"Invalid metric name","name":"`+name+`"}`, rr.Body.String())
}

func TestInvalidMethod(t *testing.T) {
	rr := testMetricUIEvent(t, http.MethodGet, "", "")
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestTimeToStartEvent(t *testing.T) {
	name := "time_to_start"
	rr := testMetricUIEvent(t, http.MethodPost, name, "0.3")
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, `{"name":"`+name+`"}`, rr.Body.String())
}

func testMetricUIEvent(t *testing.T, method, name, value string) *httptest.ResponseRecorder {
	req, err := http.NewRequest(method, "/api/v1/metric/ui", nil)
	require.NoError(t, err)

	q := req.URL.Query()
	q.Add("name", name)
	if value != "" {
		q.Add("value", value)
	}
	req.URL.RawQuery = q.Encode()

	r := mux.NewRouter()
	api.InstallRoutes(r, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}
