package test

import (
	"testing"

	jr "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
	"gotest.tools/assert"
)

func TestMockRPCServer(t *testing.T) {
	rpcServer, reqChan, nextResp := MockJSONRPCServer()
	defer rpcServer.Close()

	nextResp(`{"result": {"items": [], "page": 1, "page_size": 2, "total_pages": 3}}`)

	rsp, err := jr.NewClient(rpcServer.URL).WalletList("", 1, 1)
	if err != nil {
		t.Error(err)
	}
	<-reqChan // have to empty the chan if we're gonna do subsequent requests

	assert.Equal(t, rsp.Page, uint64(1))
	assert.Equal(t, rsp.PageSize, uint64(2))
	assert.Equal(t, rsp.TotalPages, uint64(3))
}
