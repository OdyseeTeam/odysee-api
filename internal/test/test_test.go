package test

import (
	"net/http"
	"testing"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"

	"github.com/stretchr/testify/assert"
)

func TestMockRPCServer(t *testing.T) {
	reqChan := ReqChan()
	rpcServer := MockHTTPServer(reqChan)
	defer rpcServer.Close()
	rpcServer.NextResponse <- `{"result": {"items": [], "page": 1, "page_size": 2, "total_pages": 3}}`

	res, err := ljsonrpc.NewClient(rpcServer.URL).WalletList("", 1, 2)
	if err != nil {
		t.Error(err)
	}

	req := <-reqChan
	assert.Equal(t, req.R.Method, http.MethodPost)
	assert.Equal(t, req.Body, `{"method":"wallet_list","params":{"page":1,"page_size":2},"id":0,"jsonrpc":"2.0"}`)

	assert.Equal(t, res.Page, uint64(1))
	assert.Equal(t, res.PageSize, uint64(2))
	assert.Equal(t, res.TotalPages, uint64(3))
}
