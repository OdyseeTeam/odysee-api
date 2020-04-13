package proxy

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/responses"
	"github.com/lbryio/lbrytv/internal/test"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"

	logrusTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

func newRawRequest(t *testing.T, method string, params interface{}) []byte {
	var (
		body []byte
		err  error
	)
	if params != nil {
		body, err = json.Marshal(jsonrpc.NewRequest(method, params))
	} else {
		body, err = json.Marshal(jsonrpc.NewRequest(method))
	}
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func parseRawResponse(t *testing.T, rawCallReponse []byte, v interface{}) {
	assert.NotNil(t, rawCallReponse)
	var res jsonrpc.RPCResponse
	err := json.Unmarshal(rawCallReponse, &res)
	require.NoError(t, err)
	err = res.GetObject(v)
	require.NoError(t, err)
}

func TestNewQuery(t *testing.T) {
	for _, rawQ := range []string{``, ` `, `{}`, `[]`, `[{}]`, `[""]`, `""`, `" "`, `{"method": " "}`} {
		t.Run(rawQ, func(t *testing.T) {
			q, err := NewQuery([]byte(rawQ))
			assert.Nil(t, q)
			assert.Error(t, err)
		})
	}

}

func TestNewCaller(t *testing.T) {
	servers := map[string]string{
		"first":  "http://lbrynet1",
		"second": "http://lbrynet2",
	}
	svc := NewService(sdkrouter.New(servers))
	sList := svc.SDKRouter.GetAll()
	rand.Seed(time.Now().UnixNano())
	for i := 1; i <= 100; i++ {
		id := rand.Intn(10^6-10^3) + 10 ^ 3
		wc := svc.NewCaller(fmt.Sprintf("wallet.%v", id))
		lastDigit := id % 10
		assert.Equal(t, sList[lastDigit%len(sList)].Address, wc.endpoint)
	}
}

func TestCallerСall(t *testing.T) {
	c := NewService(sdkrouter.New(config.GetLbrynetServers())).NewCaller("abc")
	for _, rawQ := range []string{``, ` `, `{}`, `[]`, `[{}]`, `[""]`, `""`, `" "`, `{"method": " "}`} {
		t.Run(rawQ, func(t *testing.T) {
			r := c.Call([]byte(rawQ))
			assert.Contains(t, string(r), `"code": -32700`)
		})
	}

}

func TestCallerSetWalletID(t *testing.T) {
	svc := NewService(sdkrouter.New(config.GetLbrynetServers()))
	c := svc.NewCaller("abc")
	assert.Equal(t, "abc", c.walletID)
}

func TestCallerCallResolve(t *testing.T) {
	svc := NewService(sdkrouter.New(config.GetLbrynetServers()))

	resolvedURL := "what#6769855a9aa43b67086f9ff3c1a5bacb5698a27a"
	resolvedClaimID := "6769855a9aa43b67086f9ff3c1a5bacb5698a27a"

	request := newRawRequest(t, "resolve", map[string]string{"urls": resolvedURL})
	rawCallReponse := svc.NewCaller("").Call(request)

	var errorResponse jsonrpc.RPCResponse
	err := json.Unmarshal(rawCallReponse, &errorResponse)
	require.NoError(t, err)
	require.Nil(t, errorResponse.Error)

	var resolveResponse ljsonrpc.ResolveResponse
	parseRawResponse(t, rawCallReponse, &resolveResponse)
	assert.Equal(t, resolvedClaimID, resolveResponse[resolvedURL].ClaimID)
}

func TestCallerCallWalletBalance(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	dummyUserID := rand.Intn(10^6-10^3) + 10 ^ 3
	rt := sdkrouter.New(config.GetLbrynetServers())
	svc := NewService(rt)

	request := newRawRequest(t, "wallet_balance", nil)

	result := svc.NewCaller("").Call(request)
	assert.Contains(t, string(result), `"message": "account identifier required"`)

	walletID, err := wallet.Create(test.RandServerAddress(t), dummyUserID)
	require.NoError(t, err)

	hook := logrusTest.NewLocal(Logger.Logger())
	result = svc.NewCaller(walletID).Call(request)

	var accountBalanceResponse ljsonrpc.AccountBalanceResponse
	parseRawResponse(t, result, &accountBalanceResponse)
	assert.EqualValues(t, "0", fmt.Sprintf("%v", accountBalanceResponse.Available))
	assert.Equal(t, map[string]interface{}{"wallet_id": fmt.Sprintf("%v", walletID)}, hook.LastEntry().Data["params"])
	assert.Equal(t, "wallet_balance", hook.LastEntry().Data["method"])
}

func TestCallerCallRelaxedMethods(t *testing.T) {
	reqChan := test.ReqChan()
	srv := test.MockHTTPServer(reqChan)
	defer srv.Close()
	caller := NewCaller(srv.URL, "")

	for _, m := range relaxedMethods {
		t.Run(m, func(t *testing.T) {
			if m == MethodStatus {
				return
			}
			srv.NextResponse <- ""
			caller.Call(newRawRequest(t, m, nil))
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
	caller := NewCaller("", "")
	for _, m := range walletSpecificMethods {
		result := caller.Call(newRawRequest(t, m, nil))
		assert.Contains(t, string(result), `"message": "account identifier required"`)
	}
}

func TestCallerCallForbiddenMethod(t *testing.T) {
	caller := NewCaller("", "")
	result := caller.Call(newRawRequest(t, "stop", nil))
	assert.Contains(t, string(result), `"message": "forbidden method"`)
}

func TestCallerCallAttachesWalletID(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	dummyWalletID := "abc123321"

	reqChan := test.ReqChan()
	srv := test.MockHTTPServer(reqChan)
	defer srv.Close()
	srv.NextResponse <- ""
	caller := NewCaller(srv.URL, dummyWalletID)
	caller.Call(newRawRequest(t, "channel_create", map[string]string{"name": "test", "bid": "0.1"}))
	receivedRequest := <-reqChan

	expectedRequest := test.ReqToStr(t, jsonrpc.RPCRequest{
		Method: "channel_create",
		Params: map[string]interface{}{
			"name":      "test",
			"bid":       "0.1",
			"wallet_id": dummyWalletID,
		},
		JSONRPC: "2.0",
	})
	assert.EqualValues(t, expectedRequest, receivedRequest.Body)
}

func TestCallerSetPreprocessor(t *testing.T) {
	reqChan := test.ReqChan()
	srv := test.MockHTTPServer(reqChan)
	defer srv.Close()

	c := NewCaller(srv.URL, "")

	c.Preprocessor = func(q *Query) {
		params := q.ParamsAsMap()
		if params == nil {
			q.Request.Params = map[string]string{"param": "123"}
		} else {
			params["param"] = "123"
			q.Request.Params = params
		}
	}

	srv.NextResponse <- ""

	c.Call(newRawRequest(t, relaxedMethods[0], nil))
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

	c := NewCaller(srv.URL, "")
	hook := logrusTest.NewLocal(Logger.Logger())
	response := c.Call(newRawRequest(t, "resolve", map[string]string{"urls": "what"}))
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
	c := NewCaller(ts.URL, "")
	response := c.Call([]byte(`{"method":"version}`))
	var rpcResponse jsonrpc.RPCResponse
	json.Unmarshal(response, &rpcResponse)
	assert.Equal(t, "2.0", rpcResponse.JSONRPC)
	assert.Equal(t, rpcErrorCodeJSONParse, rpcResponse.Error.Code)
	assert.Equal(t, "unexpected end of JSON input", rpcResponse.Error.Message)
}

func TestSDKMethodStatus(t *testing.T) {
	c := NewService(sdkrouter.New(config.GetLbrynetServers())).NewCaller("")
	callResult := c.Call(newRawRequest(t, "status", nil))
	var rpcResponse jsonrpc.RPCResponse
	json.Unmarshal(callResult, &rpcResponse)
	assert.Equal(t,
		"692EAWhtoqDuAfQ6KHMXxFxt8tkhmt7sfprEMHWKjy5hf6PwZcHDV542VHqRnFnTCD",
		rpcResponse.Result.(map[string]interface{})["installation_id"].(string))
}
