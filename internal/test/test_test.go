package test

import (
	"net/http"
	"testing"

	jr "github.com/lbryio/lbry.go/v2/extras/jsonrpc"

	"github.com/stretchr/testify/assert"
)

func TestMockRPCServer(t *testing.T) {
	reqChan := make(chan *RequestData, 1)
	rpcServer, nextResp := MockJSONRPCServer(reqChan)
	defer rpcServer.Close()
	nextResp(`{"result": {"items": [], "page": 1, "page_size": 2, "total_pages": 3}}`)

	rsp, err := jr.NewClient(rpcServer.URL).WalletList("", 1, 2)
	if err != nil {
		t.Error(err)
	}

	req := <-reqChan // read the request for inspection
	assert.Equal(t, req.Request.Method, http.MethodPost)
	assert.Equal(t, req.Body, `{"method":"wallet_list","params":{"page":1,"page_size":2},"id":0,"jsonrpc":"2.0"}`)

	assert.Equal(t, rsp.Page, uint64(1))
	assert.Equal(t, rsp.PageSize, uint64(2))
	assert.Equal(t, rsp.TotalPages, uint64(3))
}
