package status

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
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

func TestGetStatusV2_Unauthenticated(t *testing.T) {
	dbConfig := config.GetDatabase()
	params := storage.ConnParams{
		Connection: dbConfig.Connection,
		DBName:     dbConfig.DBName,
		Options:    dbConfig.Options,
	}
	c, connCleanup := storage.CreateTestConn(params)
	c.SetDefaultConnection()
	defer connCleanup()

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

	assert.Equal(t, statusOK, respStatus["general_state"])
	assert.Nil(t, respStatus["user"])
}

func TestGetStatusV2_UnauthenticatedOffline(t *testing.T) {
	dbConfig := config.GetDatabase()
	params := storage.ConnParams{
		Connection: dbConfig.Connection,
		DBName:     dbConfig.DBName,
		Options:    dbConfig.Options,
	}
	c, connCleanup := storage.CreateTestConn(params)
	c.SetDefaultConnection()
	defer connCleanup()

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
	assert.Equal(
		t,
		map[string]interface{}{
			"lbrynet": []interface{}{map[string]interface{}{
				"address": "http://malfunctioning/",
				"status":  statusOffline,
				"error":   "rpc call resolve() on http://malfunctioning/: Post \"http://malfunctioning/\": dial tcp: lookup malfunctioning: no such host",
			}},
		}, respStatus["services"])
	assert.Nil(t, respStatus["user"])
}

func TestGetStatusV2_Authenticated(t *testing.T) {
	dbConfig := config.GetDatabase()
	params := storage.ConnParams{
		Connection: dbConfig.Connection,
		DBName:     dbConfig.DBName,
		Options:    dbConfig.Options,
	}
	c, connCleanup := storage.CreateTestConn(params)
	c.SetDefaultConnection()
	defer connCleanup()

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
	assert.EqualValues(t, sdkrouter.GetSDKAddress(u), userDetails["assigned_sdk"])

	assert.Equal(
		t,
		map[string]interface{}{
			"lbrynet": []interface{}{map[string]interface{}{"address": sdkrouter.GetSDKAddress(u), "status": statusOK}},
		}, respStatus["services"])
}
