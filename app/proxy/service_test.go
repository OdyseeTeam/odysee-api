package proxy

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"

	ljsonrpc "github.com/lbryio/lbry.go/extras/jsonrpc"
	"github.com/stretchr/testify/assert"
	"github.com/ybbus/jsonrpc"
)

func prettyPrint(i interface{}) {
	s, _ := json.MarshalIndent(i, "", "\t")
	fmt.Println(string(s))
}

func newRawRequest(t *testing.T, method string, params interface{}) []byte {
	body, err := json.Marshal(jsonrpc.NewRequest(method, params))
	if err != nil {
		t.Fatal(err)
	}
	return body
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
	ps := NewProxyService()
	c := Caller{
		accountID: "abc",
		client:    DummyClient{},
		service:   ps,
	}
	assert.Equal(t, "abc", c.accountID)
	assert.Equal(t, DummyClient{}, c.client)
	assert.Equal(t, ps, c.service)
}

func TestCallerMetrics(t *testing.T) {
	ps := NewProxyService()
	c := Caller{
		client:  DummyClient{},
		service: ps,
	}
	c.Call([]byte(newRawRequest(t, "resolve", map[string]string{"urls": "what"})))
	assert.Equal(t, 0.25, math.Round(ps.GetExecTimeMetrics("resolve").ExecTime*100)/100)
}

func TestCall(t *testing.T) {
	var rpcResponse jsonrpc.RPCResponse
	var resolveResponse *ljsonrpc.ResolveResponse

	ps := NewProxyService()
	c := ps.NewCaller("")

	resolvedURL := "one#3ae4ed38414e426c29c2bd6aeab7a6ac5da74a98"
	resolvedClaimID := "3ae4ed38414e426c29c2bd6aeab7a6ac5da74a98"

	request := newRawRequest(t, "resolve", map[string]string{"urls": resolvedURL})
	rawCallReponse := c.Call(request)
	assert.NotNil(t, rawCallReponse)
	json.Unmarshal(rawCallReponse, &rpcResponse)
	rpcResponse.GetObject(&resolveResponse)
	assert.Equal(t, resolvedClaimID, (*resolveResponse)[resolvedURL].ClaimID)
	assert.True(t, ps.GetExecTimeMetrics("resolve").ExecTime > 0)
}
