package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lbryio/lbrytv/app/auth"
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/apps/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/middleware"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

func TestProxyOptions(t *testing.T) {
	r, err := http.NewRequest("OPTIONS", "/api/proxy", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	HandleCORS(rr, r)

	response := rr.Result()
	assert.Equal(t, http.StatusOK, response.StatusCode)
}

func TestProxyNilQuery(t *testing.T) {
	r, err := http.NewRequest("POST", "", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	rt := sdkrouter.New(config.GetLbrynetServers())
	handler := sdkrouter.Middleware(rt)(http.HandlerFunc(Handle))
	handler.ServeHTTP(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), `"message": "empty request body"`)
}

func TestProxyInvalidQuery(t *testing.T) {
	r, err := http.NewRequest("POST", "", bytes.NewBuffer([]byte("yo")))
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	rt := sdkrouter.New(config.GetLbrynetServers())
	handler := sdkrouter.Middleware(rt)(http.HandlerFunc(Handle))
	handler.ServeHTTP(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	var parsedResponse jsonrpc.RPCResponse
	err = json.Unmarshal(rr.Body.Bytes(), &parsedResponse)
	require.NoError(t, err)
	assert.Contains(t, parsedResponse.Error.Message, "invalid character 'y' looking for beginning of value")
}

func TestProxyDontAuthRelaxedMethods(t *testing.T) {
	var apiCalls int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls++
	}))
	config.Override("InternalAPIHost", ts.URL)
	defer config.RestoreOverridden()

	rawReq := jsonrpc.NewRequest("resolve", map[string]string{"urls": "what"})
	raw, err := json.Marshal(rawReq)
	require.NoError(t, err)

	r, err := http.NewRequest("POST", "", bytes.NewBuffer(raw))
	require.NoError(t, err)
	r.Header.Set(wallet.TokenHeader, "abc")

	rr := httptest.NewRecorder()
	rt := sdkrouter.New(config.GetLbrynetServers())
	handler := middleware.Apply(
		middleware.Chain(
			sdkrouter.Middleware(rt),
			auth.NilMiddleware,
		), Handle)
	handler.ServeHTTP(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	var parsedResponse jsonrpc.RPCResponse
	err = json.Unmarshal(rr.Body.Bytes(), &parsedResponse)
	require.NoError(t, err)
	assert.Equal(t, 0, apiCalls)
}

func Test_getOrigin(t *testing.T) {
	var r *http.Request

	r, _ = http.NewRequest(http.MethodGet, "", nil)
	r.Header.Add("Referer", "https://odysee.com/")
	assert.Equal(t, orgOdysee, getOrigin(r))

	r, _ = http.NewRequest(http.MethodGet, "", nil)
	r.Header.Add("Referer", "https://lbry.tv/")
	assert.Equal(t, orgLbrytv, getOrigin(r))

	r, _ = http.NewRequest(http.MethodGet, "", nil)
	r.Header.Add("User-Agent", "okhttp/3.12.1")
	assert.Equal(t, orgAndroid, getOrigin(r))

	r, _ = http.NewRequest(http.MethodGet, "", nil)
	r.Header.Add("User-Agent", "Odysee")
	assert.Equal(t, orgiOS, getOrigin(r))
}
