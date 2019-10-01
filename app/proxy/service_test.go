package proxy

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/lbrynet"

	ljsonrpc "github.com/lbryio/lbry.go/extras/jsonrpc"
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
	svc := NewService(endpoint)
	c := svc.NewCaller()
	assert.Equal(t, svc, c.service)
}

func TestSetWalletID(t *testing.T) {
	svc := NewService(endpoint)
	c := svc.NewCaller()
	c.SetWalletID("abc")
	assert.Equal(t, "abc", c.walletID)
}

func TestCallerMetrics(t *testing.T) {
	svc := NewService(endpoint)
	c := Caller{
		client:  &ClientMock{Delay: 250 * time.Millisecond},
		service: svc,
	}
	c.Call([]byte(newRawRequest(t, "resolve", map[string]string{"urls": "what"})))
	assert.Equal(t, 0.25, math.Round(svc.GetMetricsValue("resolve").Value*100)/100)
}

func TestCallResolve(t *testing.T) {
	var resolveResponse ljsonrpc.ResolveResponse

	svc := NewService(config.GetLbrynet())
	c := svc.NewCaller()

	resolvedURL := "one#3ae4ed38414e426c29c2bd6aeab7a6ac5da74a98"
	resolvedClaimID := "3ae4ed38414e426c29c2bd6aeab7a6ac5da74a98"

	request := newRawRequest(t, "resolve", map[string]string{"urls": resolvedURL})
	rawCallReponse := c.Call(request)
	parseRawResponse(t, rawCallReponse, &resolveResponse)
	assert.Equal(t, resolvedClaimID, resolveResponse[resolvedURL].ClaimID)
	assert.True(t, svc.GetMetricsValue("resolve").Value > 0)
}

func TestCallAccountBalance(t *testing.T) {
	// TODO: Add actual account balance response check after 0.39 support is added to lbry.go
	// var accountBalanceResponse ljsonrpc.AccountBalanceResponse

	rand.Seed(time.Now().UnixNano())
	dummyUserID := rand.Int()

	wid, _ := lbrynet.InitializeWallet(dummyUserID)

	svc := NewService(config.GetLbrynet())
	c := svc.NewCaller()
	c.SetWalletID(wid)

	request := newRawRequest(t, "account_balance", nil)
	result := c.Call(request)

	assert.Contains(t, string(result), `"message": "account identificator required"`)

	request = newRawRequest(t, "account_balance", nil)
	hook := logrus_test.NewLocal(svc.logger.Logger())
	c.Call(request)

	assert.Equal(t, map[string]interface{}{"wallet_id": fmt.Sprintf("%v", wid)}, hook.LastEntry().Data["params"])
	assert.Equal(t, "account_balance", hook.LastEntry().Data["method"])
	assert.Contains(t, string(result), `"available": "0.0"`)
}

func TestCallAttachesAccountId(t *testing.T) {
	mockClient := &ClientMock{}

	rand.Seed(time.Now().UnixNano())
	dummyWalletID := "abc123321"

	svc := NewService(endpoint)
	c := Caller{
		client:  mockClient,
		service: svc,
	}
	c.SetWalletID(dummyWalletID)
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
	wid := "walletId"

	svc := NewService("")
	client := &ClientMock{}
	c := Caller{
		client:  client,
		service: svc,
	}

	c.SetPreprocessor(func(q *Query) {
		params := q.ParamsAsMap()
		if params == nil {
			q.Request.Params = map[string]string{paramWalletID: wid}
		} else {
			params[paramWalletID] = wid
			q.Request.Params = params
		}
	})

	c.Call([]byte(newRawRequest(t, "account_list", nil)))
	p, ok := client.LastRequest.Params.(map[string]string)
	assert.True(t, ok)
	assert.Equal(t, wid, p[paramWalletID])
}

func TestCallSDKError(t *testing.T) {
	var rpcResponse jsonrpc.RPCResponse

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
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
	svc := NewService(ts.URL)
	c := svc.NewCaller()

	hook := logrus_test.NewLocal(svc.logger.Logger())
	response := c.Call([]byte(newRawRequest(t, "resolve", map[string]string{"urls": "what"})))
	json.Unmarshal(response, &rpcResponse)
	assert.Equal(t, rpcResponse.Error.Code, -32500)
	assert.Equal(t, "proxy", hook.LastEntry().Data["module"])
	assert.Equal(t, "resolve", hook.LastEntry().Data["method"])
}

func TestCallClientJSONError(t *testing.T) {
	var rpcResponse jsonrpc.RPCResponse

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"method":"version}`))
	}))
	svc := NewService(ts.URL)
	c := svc.NewCaller()

	hook := logrus_test.NewLocal(svc.logger.Logger())
	response := c.Call([]byte(`{"method":"version}`))
	json.Unmarshal(response, &rpcResponse)
	assert.Equal(t, "2.0", rpcResponse.JSONRPC)
	assert.Equal(t, ErrJSONParse, rpcResponse.Error.Code)
	assert.Equal(t, "unexpected end of JSON input", rpcResponse.Error.Message)
	assert.Equal(t, "error calling lbrynet: unexpected end of JSON input, query: {\"method\":\"version}", hook.LastEntry().Message)
}

func TestParamsAsMap(t *testing.T) {
	var q *Query

	q, _ = NewQuery(newRawRequest(t, "version", nil))
	assert.Nil(t, q.ParamsAsMap())

	q, _ = NewQuery(newRawRequest(t, "resolve", map[string]string{"urls": "what"}))
	assert.Equal(t, map[string]interface{}{"urls": "what"}, q.ParamsAsMap())

	q, _ = NewQuery(newRawRequest(t, "account_balance", nil))
	q.attachWalletID("123")
	err := q.validate()
	require.Nil(t, err)
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
