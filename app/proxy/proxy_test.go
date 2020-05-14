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
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/models"

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
	provider := func(token, ip string) (*models.User, error) { return nil, nil }
	handler := sdkrouter.Middleware(rt)(auth.Middleware(provider)(http.HandlerFunc(Handle)))
	handler.ServeHTTP(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	var parsedResponse jsonrpc.RPCResponse
	err = json.Unmarshal(rr.Body.Bytes(), &parsedResponse)
	require.NoError(t, err)
	assert.Equal(t, 0, apiCalls)
}
