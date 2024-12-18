package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/OdyseeTeam/odysee-api/api"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/Pallinder/go-randomdata"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvalidEvent(t *testing.T) {
	name := "win-the-lottery"
	rr := testMetricUIEvent(t, http.MethodPost, name, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, `{"error":"invalid metric name","name":"`+name+`"}`, rr.Body.String())
}

func TestInvalidMethod(t *testing.T) {
	rr := testMetricUIEvent(t, http.MethodGet, "", map[string]string{})
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestTimeToStartEvent(t *testing.T) {
	name := "time_to_start"
	rr := testMetricUIEvent(t, http.MethodPost, name, map[string]string{"value": "0.3"})
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, `{"name":"`+name+`"}`, rr.Body.String())

	rr = testMetricUIEvent(t, http.MethodPost, name, map[string]string{"value": "0.3", "player": "sg-p1"})
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, `{"name":"`+name+`"}`, rr.Body.String())

	rr = testMetricUIEvent(t, http.MethodPost, name, map[string]string{"value": "0.3", "player": randomdata.Alphanumeric(96)})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, `{"error":"invalid player value","name":"`+name+`"}`, rr.Body.String())
}

func testMetricUIEvent(t *testing.T, method, name string, params map[string]string) *httptest.ResponseRecorder {
	req, err := http.NewRequest(method, "/api/v1/metric/ui", nil)
	require.NoError(t, err)

	q := req.URL.Query()
	q.Add("name", name)
	for k, v := range params {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	// override this to temp to avoid permission error when running tests on
	// restricted environment.
	config.Config.Override("PublishSourceDir", os.TempDir())

	r := mux.NewRouter()
	api.InstallRoutes(r, nil, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}
