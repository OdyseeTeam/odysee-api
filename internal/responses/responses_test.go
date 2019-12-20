package responses

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

func TestJSON(t *testing.T) {
	rr := httptest.NewRecorder()
	JSON(rr, map[string]int{"error_code": 625})
	assert.Equal(t, `{"error_code":625}`, rr.Body.String())
	assert.Equal(t, "application/json; charset=utf-8", rr.Header().Get("content-type"))
	assert.Equal(t, http.StatusOK, rr.Result().StatusCode)
}

func TestJSONRPCError(t *testing.T) {
	var jResp jsonrpc.RPCResponse
	rr := httptest.NewRecorder()
	JSONRPCError(rr, "invalid input", 12345)

	err := json.Unmarshal(rr.Body.Bytes(), &jResp)
	require.NoError(t, err)

	assert.Equal(t, "invalid input", jResp.Error.Message)
	assert.Equal(t, 12345, jResp.Error.Code)
	assert.Equal(t, "application/json; charset=utf-8", rr.Header().Get("content-type"))
	assert.Equal(t, http.StatusOK, rr.Result().StatusCode)
}
