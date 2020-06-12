package status

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/lbryio/lbrytv/app/auth"
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/storage"
	"github.com/lbryio/lbrytv/internal/test"
	"github.com/lbryio/lbrytv/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// These tests requires an environment close to production setup, i.e. no loading from the config
	config.Override("LbrynetServers", map[string]string{})
	defer config.RestoreOverridden()

	dbConfig := config.GetDatabase()
	params := storage.ConnParams{
		Connection: dbConfig.Connection,
		DBName:     dbConfig.DBName,
		Options:    dbConfig.Options,
	}
	c, connCleanup := storage.CreateTestConn(params)
	c.SetDefaultConnection()

	defer connCleanup()

	os.Exit(m.Run())
}

func TestGetStatusV2_Unauthenticated(t *testing.T) {
	rr := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "", nil)
	provider := func(token, ip string) (*models.User, error) { return nil, nil }
	rt := sdkrouter.New(config.GetLbrynetServers())
	handler := sdkrouter.Middleware(rt)(auth.Middleware(provider)(http.HandlerFunc(GetStatusV2)))
	handler.ServeHTTP(rr, r)
	response := rr.Result()
	respBody, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, response.StatusCode)
	var respStatus statusResponse
	err = json.Unmarshal(respBody, &respStatus)
	require.NoError(t, err)

	assert.Equal(t, statusOK, respStatus["general_state"], respStatus)
	assert.Nil(t, respStatus["user"])
}

func TestGetStatusV2_UnauthenticatedOffline(t *testing.T) {
	_, err := models.LbrynetServers().UpdateAllG(models.M{"address": "http://malfunctioning/"})
	require.NoError(t, err)
	defer func() {
		models.LbrynetServers().UpdateAllG(models.M{"address": "http://localhost:5279/"})
	}()

	rr := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "", nil)
	provider := func(token, ip string) (*models.User, error) { return nil, nil }
	rt := sdkrouter.New(config.GetLbrynetServers())
	handler := sdkrouter.Middleware(rt)(auth.Middleware(provider)(http.HandlerFunc(GetStatusV2)))
	handler.ServeHTTP(rr, r)
	response := rr.Result()
	respBody, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, response.StatusCode)
	var respStatus statusResponse
	err = json.Unmarshal(respBody, &respStatus)
	require.NoError(t, err)

	assert.Equal(t, statusFailing, respStatus["general_state"])
	lbrynetStatus := respStatus["services"].(map[string]interface{})["lbrynet"].(map[string]interface{})
	assert.EqualValues(
		t,
		statusOffline,
		lbrynetStatus["status"],
	)
	assert.Regexp(
		t,
		"dial tcp: lookup malfunctioning",
		lbrynetStatus["error"],
	)
	assert.Nil(t, respStatus["user"])
}

func TestGetStatusV2_Authenticated(t *testing.T) {
	ts := test.MockHTTPServer(nil)
	defer ts.Close()
	ts.NextResponse <- `{
	"success": true,
	"error": null,
	"data": {
		"user_id": 123,
		"has_verified_email": true
  	}
}`

	rr := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "", nil)
	r.Header.Add(wallet.TokenHeader, "anystringwilldo")
	rt := sdkrouter.New(config.GetLbrynetServers())
	handler := sdkrouter.Middleware(rt)(auth.Middleware(auth.NewIAPIProvider(rt, ts.URL))(http.HandlerFunc(GetStatusV2)))

	handler.ServeHTTP(rr, r)
	response := rr.Result()
	respBody, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)

	u, err := wallet.GetDBUserG(123)
	require.NoError(t, err)

	var respStatus statusResponse
	err = json.Unmarshal(respBody, &respStatus)
	require.NoError(t, err)

	assert.Equal(t, statusOK, respStatus["general_state"])
	require.NotNil(t, respStatus["user"])
	userDetails := respStatus["user"].(map[string]interface{})
	assert.EqualValues(t, 123, userDetails["user_id"])
	assert.EqualValues(t, sdkrouter.GetLbrynetServer(u).Name, userDetails["assigned_sdk"])

	lbrynetStatus := respStatus["services"].(map[string]interface{})["lbrynet"].(map[string]interface{})
	assert.Equal(
		t,
		sdkrouter.GetLbrynetServer(u).Name,
		lbrynetStatus["name"],
	)
	assert.Equal(
		t,
		statusOK,
		lbrynetStatus["status"],
	)
}
