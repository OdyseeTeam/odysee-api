package proxy

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/internal/responses"
	"github.com/lbryio/lbrytv/internal/test"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"

	logrusTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

func parseRawResponse(t *testing.T, rawCallResponse []byte, v interface{}) {
	assert.NotNil(t, rawCallResponse)
	var res jsonrpc.RPCResponse
	err := json.Unmarshal(rawCallResponse, &res)
	require.NoError(t, err)
	err = res.GetObject(v)
	require.NoError(t, err)
}

func TestCallerCallRaw(t *testing.T) {
	c := NewCaller(test.RandServerAddress(t), 0)
	for _, rawQ := range []string{``, ` `, `[]`, `[{}]`, `[""]`, `""`, `" "`} {
		t.Run(rawQ, func(t *testing.T) {
			r := c.CallRaw([]byte(rawQ))
			assert.Contains(t, string(r), `"code": -32700`, `raw query: `+rawQ)
		})
	}
	for _, rawQ := range []string{`{}`, `{"method": " "}`} {
		t.Run(rawQ, func(t *testing.T) {
			r := c.CallRaw([]byte(rawQ))
			assert.Contains(t, string(r), `"code": -32080`, `raw query: `+rawQ)
		})
	}
}

func TestCallerCallResolve(t *testing.T) {
	resolvedURL := "what#6769855a9aa43b67086f9ff3c1a5bacb5698a27a"
	resolvedClaimID := "6769855a9aa43b67086f9ff3c1a5bacb5698a27a"

	request := jsonrpc.NewRequest("resolve", map[string]interface{}{"urls": resolvedURL})
	rawCallResponse := NewCaller(test.RandServerAddress(t), 0).Call(request)

	var errorResponse jsonrpc.RPCResponse
	err := json.Unmarshal(rawCallResponse, &errorResponse)
	require.NoError(t, err)
	require.Nil(t, errorResponse.Error)

	var resolveResponse ljsonrpc.ResolveResponse
	parseRawResponse(t, rawCallResponse, &resolveResponse)
	assert.Equal(t, resolvedClaimID, resolveResponse[resolvedURL].ClaimID)
}

func TestCallerCallWalletBalance(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	dummyUserID := rand.Intn(10^6-10^3) + 10 ^ 3

	request := jsonrpc.NewRequest("wallet_balance")

	result := NewCaller(test.RandServerAddress(t), 0).Call(request)
	assert.Contains(t, string(result), `"message": "authentication required"`)

	addr := test.RandServerAddress(t)
	err := wallet.Create(addr, dummyUserID)
	require.NoError(t, err)

	hook := logrusTest.NewLocal(logger.Entry.Logger)
	result = NewCaller(addr, dummyUserID).Call(request)

	var accountBalanceResponse struct {
		Available string `json:"available"`
	}
	parseRawResponse(t, result, &accountBalanceResponse)
	assert.EqualValues(t, "0.0", accountBalanceResponse.Available)
	assert.Equal(t, map[string]interface{}{"wallet_id": sdkrouter.WalletID(dummyUserID)}, hook.LastEntry().Data["params"])
	assert.Equal(t, "wallet_balance", hook.LastEntry().Data["method"])
}

func TestCallerCallRelaxedMethods(t *testing.T) {
	reqChan := test.ReqChan()
	srv := test.MockHTTPServer(reqChan)
	defer srv.Close()
	caller := NewCaller(srv.URL, 0)

	for _, m := range relaxedMethods {
		t.Run(m, func(t *testing.T) {
			if m == MethodStatus {
				return
			}
			srv.RespondWithNothing()
			caller.Call(jsonrpc.NewRequest(m))
			receivedRequest := <-reqChan
			expectedRequest := test.ReqToStr(t, jsonrpc.RPCRequest{
				Method:  m,
				Params:  nil,
				JSONRPC: "2.0",
			})
			assert.EqualValues(t, expectedRequest, receivedRequest.Body)
		})
	}
}

func TestCallerCallNonRelaxedMethods(t *testing.T) {
	caller := NewCaller("whatever", 0)
	for _, m := range walletSpecificMethods {
		result := caller.Call(jsonrpc.NewRequest(m))
		assert.Contains(t, string(result), `"message": "authentication required"`)
	}
}

func TestCallerCallForbiddenMethod(t *testing.T) {
	caller := NewCaller(test.RandServerAddress(t), 0)
	result := caller.Call(jsonrpc.NewRequest("stop"))
	assert.Contains(t, string(result), `"message": "forbidden method"`)
}

func TestCallerCallAttachesWalletID(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	dummyUserID := 123321

	reqChan := test.ReqChan()
	srv := test.MockHTTPServer(reqChan)
	defer srv.Close()
	srv.RespondWithNothing()
	caller := NewCaller(srv.URL, dummyUserID)
	caller.Call(jsonrpc.NewRequest("channel_create", map[string]interface{}{"name": "test", "bid": "0.1"}))
	receivedRequest := <-reqChan

	expectedRequest := test.ReqToStr(t, jsonrpc.RPCRequest{
		Method: "channel_create",
		Params: map[string]interface{}{
			"name":      "test",
			"bid":       "0.1",
			"wallet_id": sdkrouter.WalletID(dummyUserID),
		},
		JSONRPC: "2.0",
	})
	assert.EqualValues(t, expectedRequest, receivedRequest.Body)
}

func TestCallerSetPreprocessor(t *testing.T) {
	reqChan := test.ReqChan()
	srv := test.MockHTTPServer(reqChan)
	defer srv.Close()

	c := NewCaller(srv.URL, 0)

	c.Preprocessor = func(q *Query) {
		params := q.ParamsAsMap()
		if params == nil {
			q.Request.Params = map[string]string{"param": "123"}
		} else {
			params["param"] = "123"
			q.Request.Params = params
		}
	}

	srv.RespondWithNothing()

	c.Call(jsonrpc.NewRequest(relaxedMethods[0]))
	req := <-reqChan
	lastRequest := test.StrToReq(t, req.Body)

	p, ok := lastRequest.Params.(map[string]interface{})
	assert.True(t, ok, req.Body)
	assert.Equal(t, "123", p["param"], req.Body)
}

func TestCallerCallSDKError(t *testing.T) {
	srv := test.MockHTTPServer(nil)
	defer srv.Close()
	srv.NextResponse <- `
		{
			"jsonrpc": "2.0",
			"error": {
			  "code": -32500,
			  "message": "After successfully executing the command, failed to encode result for JSON RPC response.",
			  "data": [
				"Traceback (most recent call last):",
				"  File \"lbry/extras/daemon/Daemon.py\", line 482, in handle_old_jsonrpc",
				"  File \"lbry/extras/daemon/Daemon.py\", line 235, in jsonrpc_dumps_pretty",
				"  File \"json/__init__.py\", line 238, in dumps",
				"  File \"json/encoder.py\", line 201, in encode",
				"  File \"json/encoder.py\", line 431, in _iterencode",
				"  File \"json/encoder.py\", line 405, in _iterencode_dict",
				"  File \"json/encoder.py\", line 405, in _iterencode_dict",
				"  File \"json/encoder.py\", line 325, in _iterencode_list",
				"  File \"json/encoder.py\", line 438, in _iterencode",
				"  File \"lbry/extras/daemon/json_response_encoder.py\", line 118, in default",
				"  File \"lbry/extras/daemon/json_response_encoder.py\", line 151, in encode_output",
				"  File \"torba/client/baseheader.py\", line 75, in __getitem__",
				"  File \"lbry/wallet/header.py\", line 35, in deserialize",
				"struct.error: unpack requires a buffer of 4 bytes",
				""
			  ]
			},
			"id": 0
		}`

	c := NewCaller(srv.URL, 0)
	hook := logrusTest.NewLocal(logger.Entry.Logger)
	response := c.Call(jsonrpc.NewRequest("resolve", map[string]interface{}{"urls": "what"}))
	var rpcResponse jsonrpc.RPCResponse
	json.Unmarshal(response, &rpcResponse)
	assert.Equal(t, rpcResponse.Error.Code, -32500)
	assert.Equal(t, "proxy", hook.LastEntry().Data["module"])
	assert.Equal(t, "resolve", hook.LastEntry().Data["method"])
}

func TestCallerCallClientJSONError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responses.AddJSONContentType(w)
		w.Write([]byte(`{"method":"version}`))
	}))
	c := NewCaller(ts.URL, 0)
	response := c.CallRaw([]byte(`{"method":"version}`))
	var rpcResponse jsonrpc.RPCResponse
	json.Unmarshal(response, &rpcResponse)
	assert.Equal(t, "2.0", rpcResponse.JSONRPC)
	assert.Equal(t, rpcErrorCodeJSONParse, rpcResponse.Error.Code)
	assert.Equal(t, "unexpected end of JSON input", rpcResponse.Error.Message)
}

func TestSDKMethodStatus(t *testing.T) {
	c := NewCaller(test.RandServerAddress(t), 0)
	callResult := c.Call(jsonrpc.NewRequest("status"))
	var rpcResponse jsonrpc.RPCResponse
	json.Unmarshal(callResult, &rpcResponse)
	assert.Equal(t,
		"692EAWhtoqDuAfQ6KHMXxFxt8tkhmt7sfprEMHWKjy5hf6PwZcHDV542VHqRnFnTCD",
		rpcResponse.Result.(map[string]interface{})["installation_id"].(string))
}
