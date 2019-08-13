package proxy

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	ljsonrpc "github.com/lbryio/lbry.go/extras/jsonrpc"
	"github.com/lbryio/lbrytv/internal/lbrynet"
	"github.com/stretchr/testify/assert"
	"github.com/ybbus/jsonrpc"
)

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
	return nil, nil
}

func (c DummyClient) CallRaw(request *jsonrpc.RPCRequest) (*jsonrpc.RPCResponse, error) {
	time.Sleep(250 * time.Millisecond)
	return nil, nil
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
	srv := NewService()
	c := srv.NewCaller()
	assert.Equal(t, srv, c.service)
}

func TestSetAccountID(t *testing.T) {
	srv := NewService()
	c := srv.NewCaller()
	c.SetAccountID("abc")
	assert.Equal(t, "abc", c.accountID)
}

func TestCallerMetrics(t *testing.T) {
	srv := NewService()
	c := Caller{
		client:  DummyClient{},
		service: srv,
	}
	c.Call([]byte(newRawRequest(t, "resolve", map[string]string{"urls": "what"})))
	assert.Equal(t, 0.25, math.Round(srv.GetExecTimeMetrics("resolve").ExecTime*100)/100)
}

func TestCallResolve(t *testing.T) {
	var resolveResponse ljsonrpc.ResolveResponse

	srv := NewService()
	c := srv.NewCaller()

	resolvedURL := "one#3ae4ed38414e426c29c2bd6aeab7a6ac5da74a98"
	resolvedClaimID := "3ae4ed38414e426c29c2bd6aeab7a6ac5da74a98"

	request := newRawRequest(t, "resolve", map[string]string{"urls": resolvedURL})
	rawCallReponse := c.Call(request)
	parseRawResponse(t, rawCallReponse, &resolveResponse)
	assert.Equal(t, resolvedClaimID, resolveResponse[resolvedURL].ClaimID)
	assert.True(t, srv.GetExecTimeMetrics("resolve").ExecTime > 0)
}

func TestCallBalance(t *testing.T) {
	var accResponse ljsonrpc.Account

	rand.Seed(time.Now().UnixNano())
	dummyAccountID := rand.Int()

	acc, _ := lbrynet.CreateAccount(dummyAccountID)
	defer lbrynet.RemoveAccount(dummyAccountID)

	srv := NewService()
	c := srv.NewCaller()
	c.SetAccountID(acc.ID)

	request := newRawRequest(t, "account_list", nil)
	rawCallReponse := c.Call(request)
	parseRawResponse(t, rawCallReponse, &accResponse)
	assert.Equal(t, acc.ID, accResponse.ID)
	assert.True(t, srv.GetExecTimeMetrics("account_list").ExecTime > 0)
}
