package status

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/lbryio/lbrytv/app/auth"
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/apps/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/middleware"
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

	code := m.Run()
	connCleanup()

	os.Exit(code)
}

func TestGetStatusV2_Unauthenticated(t *testing.T) {
	rr := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "", nil)
	rt := sdkrouter.New(config.GetLbrynetServers())
	handler := middleware.Apply(
		middleware.Chain(
			sdkrouter.Middleware(rt),
			auth.NilMiddleware,
		), GetStatusV2)

	handler.ServeHTTP(rr, r)
	response := rr.Result()
	respBody, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, response.StatusCode)
	var respStatus statusResponse
	err = json.Unmarshal(respBody, &respStatus)
	require.NoError(t, err)

	assert.Equal(t, statusOK, respStatus.GeneralState, respStatus)
	assert.Nil(t, respStatus.User)
}

func TestGetStatusV2_UnauthenticatedOffline(t *testing.T) {
	_, err := models.LbrynetServers().UpdateAllG(models.M{"address": "http://malfunctioning/"})
	require.NoError(t, err)
	defer func() {
		models.LbrynetServers().UpdateAllG(models.M{"address": "http://localhost:5279/"})
	}()

	rr := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "", nil)
	rt := sdkrouter.New(config.GetLbrynetServers())
	handler := middleware.Apply(
		middleware.Chain(
			sdkrouter.Middleware(rt),
			auth.NilMiddleware,
		), GetStatusV2)

	handler.ServeHTTP(rr, r)
	response := rr.Result()
	respBody, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, response.StatusCode)
	var respStatus statusResponse
	err = json.Unmarshal(respBody, &respStatus)
	require.NoError(t, err)

	assert.Equal(t, statusFailing, respStatus.GeneralState)
	fmt.Printf("%v", respStatus)
	lbrynetStatus := respStatus.Services["lbrynet"][0]
	assert.EqualValues(
		t,
		statusOffline,
		lbrynetStatus.Status,
	)
	assert.Regexp(
		t,
		"dial tcp: lookup malfunctioning",
		lbrynetStatus.Error,
	)
	assert.Nil(t, respStatus.User)
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
	handler := middleware.Apply(
		middleware.Chain(
			sdkrouter.Middleware(rt),
			auth.MiddlewareWithProvider(rt, ts.URL),
		), GetStatusV2)

	handler.ServeHTTP(rr, r)
	response := rr.Result()
	respBody, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)

	u, err := wallet.GetDBUserG(wallet.ByID(123))
	require.NoError(t, err)

	var respStatus statusResponse
	err = json.Unmarshal(respBody, &respStatus)
	require.NoError(t, err)

	assert.Equal(t, statusOK, respStatus.GeneralState)
	require.NotNil(t, respStatus.User)
	assert.EqualValues(t, 123, respStatus.User.ID)
	assert.EqualValues(t, sdkrouter.GetLbrynetServer(u).Name, respStatus.User.AssignedLbrynetServer)

	lbrynetStatus := respStatus.Services["lbrynet"][0]
	assert.Equal(
		t,
		sdkrouter.GetLbrynetServer(u).Name,
		lbrynetStatus.Name,
	)
	assert.Equal(
		t,
		statusOK,
		lbrynetStatus.Status,
	)
}
