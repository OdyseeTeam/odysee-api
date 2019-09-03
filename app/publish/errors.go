package publish

import (
	"encoding/json"
	"github.com/lbryio/lbrytv/app/proxy"

	"github.com/ybbus/jsonrpc"
)

type Error struct {
	code    int
	message string
}

func (e Error) AsRPCResponse() *jsonrpc.RPCResponse {
	return &jsonrpc.RPCResponse{
		Error: &jsonrpc.RPCError{
			Code:    e.Code(),
			Message: e.Message(),
		},
		JSONRPC: "2.0",
	}
}

func (e Error) AsBytes() []byte {
	b, _ := json.MarshalIndent(e.AsRPCResponse(), "", "  ")
	return b
}

func (e Error) Code() int {
	return e.code
}

func (e Error) Message() string {
	return e.message
}

var ErrUnauthorized = Error{code: proxy.ErrProxy, message: "authentication required"}

func NewAuthError(err error) Error {
	return Error{code: proxy.ErrAuthFailed, message: err.Error()}
}
