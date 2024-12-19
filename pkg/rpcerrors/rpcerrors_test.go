package rpcerrors

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc/v2"
)

func TestWrite(t *testing.T) {
	w := new(bytes.Buffer)
	Write(w, errors.New("error!"))

	b, _ := json.MarshalIndent(jsonrpc.RPCResponse{
		Error: &jsonrpc.RPCError{
			Code:    rpcErrorCodeInternal,
			Message: "error!",
		},
		JSONRPC: "2.0",
	}, "", "  ")
	require.Equal(t, b, w.Bytes())
}
