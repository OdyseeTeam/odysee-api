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

type DummyClient struct{}

func (c DummyClient) Call(method string, params ...interface{}) (*jsonrpc.RPCResponse, error) {
	return &jsonrpc.RPCResponse{
		JSONRPC: "2.0",
		Result:  "0.0",
	}, nil
}

func (c DummyClient) CallRaw(request *jsonrpc.RPCRequest) (*jsonrpc.RPCResponse, error) {
	time.Sleep(250 * time.Millisecond)
	return &jsonrpc.RPCResponse{
		JSONRPC: "2.0",
		Result:  "0.0",
	}, nil
}

func (c DummyClient) CallFor(out interface{}, method string, params ...interface{}) error {
	return nil
}

func (c DummyClient) CallBatch(requests jsonrpc.RPCRequests) (jsonrpc.RPCResponses, error) {
	return nil, nil
}

func (c DummyClient) CallBatchRaw(requests jsonrpc.RPCRequests) (jsonrpc.RPCResponses, error) {
	return nil, nil
}

func TestNewCaller(t *testing.T) {
	svc := NewService(endpoint)
	c := svc.NewCaller()
	assert.Equal(t, svc, c.service)
}

func TestSetAccountID(t *testing.T) {
	svc := NewService(endpoint)
	c := svc.NewCaller()
	c.SetAccountID("abc")
	assert.Equal(t, "abc", c.accountID)
}

func TestCallerMetrics(t *testing.T) {
	svc := NewService(endpoint)
	c := Caller{
		client:  DummyClient{},
		service: svc,
	}
	c.Call([]byte(newRawRequest(t, "resolve", map[string]string{"urls": "what"})))
	assert.Equal(t, 0.25, math.Round(svc.GetExecTimeMetrics("resolve").ExecTime*100)/100)
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
	assert.True(t, svc.GetExecTimeMetrics("resolve").ExecTime > 0)
}

func TestCallAccountBalance(t *testing.T) {
	// TODO: Add actual account balance response check after 0.39 support is added to lbry.go
	// var accountBalanceResponse ljsonrpc.AccountBalanceResponse

	rand.Seed(time.Now().UnixNano())
	dummyAccountID := rand.Int()

	acc, _ := lbrynet.CreateAccount(dummyAccountID)
	defer lbrynet.RemoveAccount(dummyAccountID)

	svc := NewService(config.GetLbrynet())
	c := svc.NewCaller()
	c.SetAccountID(acc.ID)

	request := newRawRequest(t, "account_balance", nil)
	hook := logrus_test.NewLocal(svc.logger.Logger())
	c.Call(request)

	assert.Equal(t, map[string]interface{}{"account_id": "****"}, hook.LastEntry().Data["params"])
	assert.Equal(t, "account_balance", hook.LastEntry().Data["method"])
}

func TestCallAccountList(t *testing.T) {
	var accResponse ljsonrpc.Account

	rand.Seed(time.Now().UnixNano())
	dummyAccountID := rand.Int()

	acc, _ := lbrynet.CreateAccount(dummyAccountID)
	defer lbrynet.RemoveAccount(dummyAccountID)

	svc := NewService(config.GetLbrynet())
	c := svc.NewCaller()
	c.SetAccountID(acc.ID)

	request := newRawRequest(t, "account_list", nil)
	rawCallReponse := c.Call(request)
	parseRawResponse(t, rawCallReponse, &accResponse)
	assert.Equal(t, acc.ID, accResponse.ID)
	assert.True(t, svc.GetExecTimeMetrics("account_list").ExecTime > 0)
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
	assert.Equal(t, "malformed JSON from client: unexpected end of JSON input", hook.LastEntry().Message)
}
