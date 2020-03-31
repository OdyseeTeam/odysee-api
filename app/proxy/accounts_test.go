package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lbryio/lbrytv/config"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

func TestWithWrongAuthToken(t *testing.T) {
	testFuncSetup()
	defer testFuncTeardown()

	var (
		q        *jsonrpc.RPCRequest
		qBody    []byte
		response jsonrpc.RPCResponse
	)

	ts := launchDummyAPIServer([]byte(`{
		"success": false,
		"error": "could not authenticate user",
		"data": null
	}`))
	defer ts.Close()
	config.Override("InternalAPIHost", ts.URL)
	defer config.RestoreOverridden()

	q = jsonrpc.NewRequest("account_list")
	qBody, _ = json.Marshal(q)
	r, _ := http.NewRequest("POST", proxySuffix, bytes.NewBuffer(qBody))
	r.Header.Add("X-Lbry-Auth-Token", "xXxXxXx")

	rr := httptest.NewRecorder()
	handler := NewRequestHandler(svc)
	handler.Handle(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.Nil(t, err)
	assert.Equal(t, "cannot authenticate user with internal-apis: could not authenticate user", response.Error.Message)
}

func TestWithoutToken(t *testing.T) {
	testFuncSetup()
	defer testFuncTeardown()

	var (
		q              *jsonrpc.RPCRequest
		qBody          []byte
		response       jsonrpc.RPCResponse
		statusResponse ljsonrpc.StatusResponse
	)

	q = jsonrpc.NewRequest("status")
	qBody, _ = json.Marshal(q)
	r, _ := http.NewRequest("POST", proxySuffix, bytes.NewBuffer(qBody))

	rr := httptest.NewRecorder()
	handler := NewRequestHandler(svc)
	handler.Handle(rr, r)
	require.Equal(t, http.StatusOK, rr.Code)

	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.Nil(t, err)
	require.Nil(t, response.Error)

	err = ljsonrpc.Decode(response.Result, &statusResponse)
	require.Nil(t, err)
	assert.True(t, statusResponse.IsRunning)
}

func TestAccountSpecificWithoutToken(t *testing.T) {
	testFuncSetup()
	defer testFuncTeardown()

	var (
		q        *jsonrpc.RPCRequest
		qBody    []byte
		response jsonrpc.RPCResponse
	)

	q = jsonrpc.NewRequest("account_list")
	qBody, _ = json.Marshal(q)
	r, _ := http.NewRequest("POST", proxySuffix, bytes.NewBuffer(qBody))

	rr := httptest.NewRecorder()
	handler := NewRequestHandler(svc)
	handler.Handle(rr, r)
	require.Equal(t, http.StatusOK, rr.Code)

	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.Nil(t, err)
	require.NotNil(t, response.Error)
	require.Equal(t, "account identificator required", response.Error.Message)
}
