package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/app/router"

	"github.com/lbryio/lbrytv/internal/lbrynet"
	"github.com/lbryio/lbrytv/internal/responses"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
	logrus_test "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

const endpoint = "http://localhost:5279/"

func prettyPrint(i interface{}) {
	s, _ := json.MarshalIndent(i, "", "\t")
	fmt.Println(string(s))
}

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

func parseRawResponse(t *testing.T, rawCallReponse []byte, destinationVar interface{}) {
	var rpcResponse jsonrpc.RPCResponse

	assert.NotNil(t, rawCallReponse)

	json.Unmarshal(rawCallReponse, &rpcResponse)
	rpcResponse.GetObject(destinationVar)
}

type ClientMock struct {
	Delay       time.Duration
	LastRequest jsonrpc.RPCRequest
}

func (c ClientMock) Call(method string, params ...interface{}) (*jsonrpc.RPCResponse, error) {
	return &jsonrpc.RPCResponse{
		JSONRPC: "2.0",
		Result:  "0.0",
	}, nil
}

func (c *ClientMock) CallRaw(request *jsonrpc.RPCRequest) (*jsonrpc.RPCResponse, error) {
	c.LastRequest = *request
	time.Sleep(c.Delay)
	return &jsonrpc.RPCResponse{
		JSONRPC: "2.0",
		Result:  "0.0",
	}, nil
}

func (c ClientMock) CallFor(out interface{}, method string, params ...interface{}) error {
	return nil
}

func (c ClientMock) CallBatch(requests jsonrpc.RPCRequests) (jsonrpc.RPCResponses, error) {
	return nil, nil
}

func (c ClientMock) CallBatchRaw(requests jsonrpc.RPCRequests) (jsonrpc.RPCResponses, error) {
	return nil, nil
}

func TestNewCaller(t *testing.T) {
	servers := map[string]string{
		"default": "http://lbrynet1",
		"second":  "http://lbrynet2",
	}
	svc := NewService(Opts{SDKRouter: router.New(servers)})
	c := svc.NewCaller("")
	assert.Equal(t, svc, c.service)

	sList := svc.Router.GetSDKServerList()
	rand.Seed(time.Now().UnixNano())
	for i := 1; i <= 100; i++ {
		id := rand.Intn(10^6-10^3) + 10 ^ 3
		wc := svc.NewCaller(fmt.Sprintf("wallet.%v", id))
		lastDigit := id % 10
		assert.Equal(t, sList[lastDigit%len(sList)].Address, wc.endpoint)
	}
}

func TestCallerSetWalletID(t *testing.T) {
	svc := NewService(Opts{SDKRouter: router.NewDefault()})
	c := svc.NewCaller("abc")
	assert.Equal(t, "abc", c.walletID)
}

func TestCallerCallResolve(t *testing.T) {
	var (
		errorResponse   jsonrpc.RPCResponse
		resolveResponse ljsonrpc.ResolveResponse
	)

	svc := NewService(Opts{SDKRouter: router.NewDefault()})
	c := svc.NewCaller("")

	resolvedURL := "what#6769855a9aa43b67086f9ff3c1a5bacb5698a27a"
	resolvedClaimID := "6769855a9aa43b67086f9ff3c1a5bacb5698a27a"

	request := newRawRequest(t, "resolve", map[string]string{"urls": resolvedURL})
	rawCallReponse := c.Call(request)
	err := json.Unmarshal(rawCallReponse, &errorResponse)
	require.NoError(t, err)
	require.Nil(t, errorResponse.Error)

	parseRawResponse(t, rawCallReponse, &resolveResponse)
	assert.Equal(t, resolvedClaimID, resolveResponse[resolvedURL].ClaimID)
}

func TestCallerCallWalletBalance(t *testing.T) {
	var accountBalanceResponse ljsonrpc.AccountBalanceResponse

	rand.Seed(time.Now().UnixNano())
	dummyUserID := rand.Intn(10^6-10^3) + 10 ^ 3

	_, wid, err := lbrynet.InitializeWallet(dummyUserID)
	require.NoError(t, err)

	svc := NewService(Opts{SDKRouter: router.NewDefault()})
	request := newRawRequest(t, "wallet_balance", nil)

	c := svc.NewCaller("")
	result := c.Call(request)
	assert.Contains(t, string(result), `"message": "account identificator required"`)

	c = svc.NewCaller(wid)
	hook := logrus_test.NewLocal(svc.logger.Logger())
	result = c.Call(request)

	parseRawResponse(t, result, &accountBalanceResponse)
	assert.EqualValues(t, "0", fmt.Sprintf("%v", accountBalanceResponse.Available))
	assert.Equal(t, map[string]interface{}{"wallet_id": fmt.Sprintf("%v", wid)}, hook.LastEntry().Data["params"])
	assert.Equal(t, "wallet_balance", hook.LastEntry().Data["method"])
}

func TestCallerCallDoesReloadWallet(t *testing.T) {
	var (
		response jsonrpc.RPCResponse
	)

	rand.Seed(time.Now().UnixNano())
	dummyUserID := rand.Intn(100)

	_, wid, _ := lbrynet.InitializeWallet(dummyUserID)
	_, err := lbrynet.WalletRemove(dummyUserID)
	require.NoError(t, err)

	svc := NewService(Opts{SDKRouter: router.NewDefault()})
	c := svc.NewCaller(wid)

	request := newRawRequest(t, "wallet_balance", nil)
	result := c.Call(request)

	assert.Equal(t, walletLoadRetries-1, c.retries)
	err = json.Unmarshal(result, &response)
	require.NoError(t, err)
	require.Nil(t, response.Error)
}

func TestCallerCallRelaxedMethods(t *testing.T) {
	for _, m := range relaxedMethods {
		t.Run(m, func(t *testing.T) {
			if m == MethodStatus {
				return
			}
			mockClient := &ClientMock{}
			svc := NewService(Opts{SDKRouter: router.NewDefault()})
			c := Caller{
				client:  mockClient,
				service: svc,
			}
			request := newRawRequest(t, m, nil)
			result := c.Call(request)
			expectedRequest := jsonrpc.RPCRequest{
				Method:  m,
				Params:  nil,
				JSONRPC: "2.0",
			}
			assert.EqualValues(t, expectedRequest, mockClient.LastRequest, string(result))
		})
	}
}

func TestCallerCallNonRelaxedMethods(t *testing.T) {
	for _, m := range walletSpecificMethods {
		mockClient := &ClientMock{}
		svc := NewService(Opts{SDKRouter: router.NewDefault()})
		c := Caller{
			client:  mockClient,
			service: svc,
		}
		request := newRawRequest(t, m, nil)
		result := c.Call(request)
		assert.Contains(t, string(result), `"message": "account identificator required"`)
	}
}

func TestCallerCallForbiddenMethod(t *testing.T) {
	mockClient := &ClientMock{}
	svc := NewService(Opts{SDKRouter: router.NewDefault()})
	c := Caller{
		client:  mockClient,
		service: svc,
	}
	request := newRawRequest(t, "stop", nil)
	result := c.Call(request)
	assert.Contains(t, string(result), `"message": "forbidden method"`)
}

func TestCallerCallAttachesWalletID(t *testing.T) {
	mockClient := &ClientMock{}

	rand.Seed(time.Now().UnixNano())
	dummyWalletID := "abc123321"

	svc := NewService(Opts{SDKRouter: router.NewDefault()})
	c := Caller{
		walletID: dummyWalletID,
		client:   mockClient,
		service:  svc,
	}
	c.Call([]byte(newRawRequest(t, "channel_create", map[string]string{"name": "test", "bid": "0.1"})))
	expectedRequest := jsonrpc.RPCRequest{
		Method: "channel_create",
		Params: map[string]interface{}{
			"name":      "test",
			"bid":       "0.1",
			"wallet_id": dummyWalletID,
		},
		JSONRPC: "2.0",
	}
	assert.EqualValues(t, expectedRequest, mockClient.LastRequest)
}

func TestCallerSetPreprocessor(t *testing.T) {
	svc := NewService(Opts{SDKRouter: router.NewDefault()})
	client := &ClientMock{}
	c := Caller{
		client:  client,
		service: svc,
	}

	c.SetPreprocessor(func(q *Query) {
		params := q.ParamsAsMap()
		if params == nil {
			q.Request.Params = map[string]string{"param": "123"}
		} else {
			params["param"] = "123"
			q.Request.Params = params
		}
	})

	c.Call([]byte(newRawRequest(t, relaxedMethods[0], nil)))
	p, ok := client.LastRequest.Params.(map[string]string)
	assert.True(t, ok)
	assert.Equal(t, "123", p["param"])
}

func TestCallerCallSDKError(t *testing.T) {
	var rpcResponse jsonrpc.RPCResponse

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responses.PrepareJSONWriter(w)
		w.Write([]byte(`
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
		}
		`))
	}))
	svc := NewService(Opts{SDKRouter: router.New(router.SingleLbrynetServer(ts.URL))})
	c := svc.NewCaller("")

	hook := logrus_test.NewLocal(svc.logger.Logger())
	response := c.Call([]byte(newRawRequest(t, "resolve", map[string]string{"urls": "what"})))
	json.Unmarshal(response, &rpcResponse)
	assert.Equal(t, rpcResponse.Error.Code, -32500)
	assert.Equal(t, "proxy", hook.LastEntry().Data["module"])
	assert.Equal(t, "resolve", hook.LastEntry().Data["method"])
}

func TestCallerCallClientJSONError(t *testing.T) {
	var rpcResponse jsonrpc.RPCResponse

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responses.PrepareJSONWriter(w)
		w.Write([]byte(`{"method":"version}`))
	}))
	svc := NewService(Opts{SDKRouter: router.New(router.SingleLbrynetServer(ts.URL))})
	c := svc.NewCaller("")

	hook := logrus_test.NewLocal(svc.logger.Logger())
	response := c.Call([]byte(`{"method":"version}`))
	json.Unmarshal(response, &rpcResponse)
	assert.Equal(t, "2.0", rpcResponse.JSONRPC)
	assert.Equal(t, ErrJSONParse, rpcResponse.Error.Code)
	assert.Equal(t, "unexpected end of JSON input", rpcResponse.Error.Message)
	assert.Equal(t, "error calling lbrynet: unexpected end of JSON input, query: {\"method\":\"version}", hook.LastEntry().Message)
}

func TestQueryParamsAsMap(t *testing.T) {
	var q *Query

	q, _ = NewQuery(newRawRequest(t, "version", nil))
	assert.Nil(t, q.ParamsAsMap())

	q, _ = NewQuery(newRawRequest(t, "resolve", map[string]string{"urls": "what"}))
	assert.Equal(t, map[string]interface{}{"urls": "what"}, q.ParamsAsMap())

	q, _ = NewQuery(newRawRequest(t, "account_balance", nil))
	q.SetWalletID("123")
	err := q.validate()
	require.Nil(t, err, errors.Unwrap(err))
	assert.Equal(t, map[string]interface{}{"wallet_id": "123"}, q.ParamsAsMap())

	searchParams := map[string]interface{}{
		"any_tags": []interface{}{
			"art", "automotive", "blockchain", "comedy", "economics", "education",
			"gaming", "music", "news", "science", "sports", "technology",
		},
	}
	q, _ = NewQuery(newRawRequest(t, "claim_search", searchParams))
	assert.Equal(t, searchParams, q.ParamsAsMap())
}

func TestSDKMethodStatus(t *testing.T) {
	var rpcResponse jsonrpc.RPCResponse

	svc := NewService(Opts{SDKRouter: router.NewDefault()})
	c := svc.NewCaller("")
	request := newRawRequest(t, "status", nil)
	callResult := c.Call(request)

	json.Unmarshal(callResult, &rpcResponse)
	result := rpcResponse.Result.(map[string]interface{})
	assert.Equal(t,
		"692EAWhtoqDuAfQ6KHMXxFxt8tkhmt7sfprEMHWKjy5hf6PwZcHDV542VHqRnFnTCD",
		result["installation_id"].(string))
}
