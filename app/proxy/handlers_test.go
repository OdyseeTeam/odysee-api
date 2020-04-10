package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lbryio/lbrytv/app/users"
	"github.com/lbryio/lbrytv/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

func TestProxyOptions(t *testing.T) {
	r, _ := http.NewRequest("OPTIONS", "/api/proxy", nil)

	rr := httptest.NewRecorder()
	handler := NewRequestHandler(svc)
	handler.HandleOptions(rr, r)

	response := rr.Result()
	assert.Equal(t, http.StatusOK, response.StatusCode)
}

func TestProxyNilQuery(t *testing.T) {
	r, _ := http.NewRequest("POST", "", nil)

	rr := httptest.NewRecorder()
	handler := NewRequestHandler(svc)
	handler.Handle(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "empty request body", rr.Body.String())
}

func TestProxyInvalidQuery(t *testing.T) {
	var parsedResponse jsonrpc.RPCResponse

	r, _ := http.NewRequest("POST", "", bytes.NewBuffer([]byte("yo")))

	rr := httptest.NewRecorder()
	handler := NewRequestHandler(svc)
	handler.Handle(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	err := json.Unmarshal(rr.Body.Bytes(), &parsedResponse)
	require.NoError(t, err)
	assert.Contains(t, parsedResponse.Error.Message, "invalid character 'y' looking for beginning of value")
}

func TestProxyDontAuthRelaxedMethods(t *testing.T) {
	var parsedResponse jsonrpc.RPCResponse
	var apiCalls int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls++
	}))
	config.Override("InternalAPIHost", ts.URL)

	r, _ := http.NewRequest("POST", "", bytes.NewBuffer([]byte(newRawRequest(t, "resolve", map[string]string{"urls": "what"}))))
	r.Header.Set(users.TokenHeader, "abc")

	rr := httptest.NewRecorder()
	handler := NewRequestHandler(svc)
	handler.Handle(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	err := json.Unmarshal(rr.Body.Bytes(), &parsedResponse)
	require.NoError(t, err)

	assert.Equal(t, 0, apiCalls)
}
