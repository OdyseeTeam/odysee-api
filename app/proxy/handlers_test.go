package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
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
	r, _ := http.NewRequest("POST", "/api/proxy", nil)

	rr := httptest.NewRecorder()
	handler := NewRequestHandler(svc)
	handler.Handle(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "empty request body", rr.Body.String())
}

func TestProxyInvalidQuery(t *testing.T) {
	var parsedResponse jsonrpc.RPCResponse

	r, _ := http.NewRequest("POST", "/api/proxy", bytes.NewBuffer([]byte("yo")))

	rr := httptest.NewRecorder()
	handler := NewRequestHandler(svc)
	handler.Handle(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	err := json.Unmarshal(rr.Body.Bytes(), &parsedResponse)
	if err != nil {
		panic(err)
	}
	assert.Contains(t, parsedResponse.Error.Message, "invalid character 'y' looking for beginning of value")
}
