package lbrynext

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/ybbus/jsonrpc"
)

func Test_compareResponses(t *testing.T) {
	r := &jsonrpc.RPCResponse{Result: map[string]string{"ok": "ok"}}
	xr := &jsonrpc.RPCResponse{Result: map[string]string{"ok": "not ok"}}
	_, _, diff := compareResponses(r, xr)
	fmt.Println(diffPlainText(diff))
	assert.Contains(t, diffPlainText(diff), `"ok": "+>>not <<+ok"`)
}
