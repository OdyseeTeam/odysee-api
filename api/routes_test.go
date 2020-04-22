package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/lbryio/lbrytv/app/publish"
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

func TestRoutesProxy(t *testing.T) {
	r := mux.NewRouter()
	rt := sdkrouter.New(config.GetLbrynetServers())

	req, err := http.NewRequest("POST", "/api/v1/proxy", bytes.NewBuffer([]byte(`{"method": "status"}`)))
	require.NoError(t, err)
	rr := httptest.NewRecorder()

	InstallRoutes(r, rt)
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"result":`)
}

func TestRoutesPublish(t *testing.T) {
	r := mux.NewRouter()
	rt := sdkrouter.New(config.GetLbrynetServers())

	req := publish.CreatePublishRequest(t, []byte("test file"))
	rr := httptest.NewRecorder()

	InstallRoutes(r, rt)
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	// Authentication Required error here is enough to see that the request
	// has been dispatched through the publish handler
	assert.Contains(t, rr.Body.String(), `"code": -32084`)
}

func TestRoutesOptions(t *testing.T) {
	r := mux.NewRouter()
	rt := sdkrouter.New(config.GetLbrynetServers())

	req, err := http.NewRequest("OPTIONS", "/api/v1/proxy", nil)
	require.NoError(t, err)
	rr := httptest.NewRecorder()

	InstallRoutes(r, rt)
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "7200", rr.Result().Header.Get("Access-Control-Max-Age"))
	assert.Equal(t, "*", rr.Result().Header.Get("Access-Control-Allow-Origin"))
	assert.Equal(
		t,
		"X-Lbry-Auth-Token, Origin, X-Requested-With, Content-Type, Accept",
		rr.Result().Header.Get("Access-Control-Allow-Headers"),
	)
}

func TestRecoveryHandler_Panic(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("xoxox")
	})
	rr := httptest.NewRecorder()
	r, err := http.NewRequest(http.MethodGet, "/", &bytes.Buffer{})
	require.NoError(t, err)
	logger.Disable()
	assert.NotPanics(t, func() {
		recoveryHandler(h).ServeHTTP(rr, r)
	})
	var res jsonrpc.RPCResponse
	err = json.Unmarshal(rr.Body.Bytes(), &res)
	require.NoError(t, err)
	require.NotNil(t, res.Error)
	assert.Contains(t, res.Error.Message, "xoxox")
}

func TestRecoveryHandler_NoPanic(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("no panic recovery"))
	})
	rr := httptest.NewRecorder()
	r, err := http.NewRequest(http.MethodGet, "/", &bytes.Buffer{})
	require.NoError(t, err)
	assert.NotPanics(t, func() {
		recoveryHandler(h).ServeHTTP(rr, r)
	})
	assert.Equal(t, rr.Body.String(), "no panic recovery")

}

func TestRecoveryHandler_RecoveredPanic(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if r := recover(); r != nil {
				w.Write([]byte("panic recovered in here"))
			}
		}()
		panic("xoxoxo")
	})
	rr := httptest.NewRecorder()
	r, err := http.NewRequest(http.MethodGet, "/", &bytes.Buffer{})
	require.NoError(t, err)
	assert.NotPanics(t, func() {
		recoveryHandler(h).ServeHTTP(rr, r)
	})
	assert.Equal(t, rr.Body.String(), "panic recovered in here")
}
