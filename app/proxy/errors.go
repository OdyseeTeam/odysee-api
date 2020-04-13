package proxy

import (
	"encoding/json"
	"errors"

	"github.com/ybbus/jsonrpc"
)

const (
	rpcErrorCodeInternal         int = -32080 // general errors that originate inside the proxy module
	rpcErrorCodeSDK              int = -32603 // otherwise-unspecified errors from the SDK
	rpcErrorCodeAuthRequired     int = -32084 // auth info is required but is not provided
	rpcErrorCodeUnauthorized     int = -32085 // auth info is provided but is not found in the database
	rpcErrorCodeJSONParse        int = -32700 // invalid JSON was received by the server
	rpcErrorCodeInvalidParams    int = -32602 // error in params that the client provided
	rpcErrorCodeMethodNotAllowed int = -32601 // the requested method is not allowed to be called
)

type RPCError struct {
	err  error
	code int
}

func (e RPCError) Error() string { return e.err.Error() }
func (e RPCError) Code() int     { return e.code }
func (e RPCError) Unwrap() error { return e.err }

func (e RPCError) JSON() []byte {
	b, err := json.MarshalIndent(jsonrpc.RPCResponse{
		Error: &jsonrpc.RPCError{
			Code:    e.Code(),
			Message: e.Error(),
		},
		JSONRPC: "2.0",
	}, "", "  ")
	if err != nil {
		Logger.Errorf("rpc error to json: %v", err)
	}
	return b
}

func NewInternalError(e error) RPCError         { return RPCError{e, rpcErrorCodeInternal} }
func NewJSONParseError(e error) RPCError        { return RPCError{e, rpcErrorCodeJSONParse} }
func NewMethodNotAllowedError(e error) RPCError { return RPCError{e, rpcErrorCodeMethodNotAllowed} }
func NewInvalidParamsError(e error) RPCError    { return RPCError{e, rpcErrorCodeInvalidParams} }
func NewSDKError(e error) RPCError              { return RPCError{e, rpcErrorCodeSDK} }
func NewUnauthorizedError(e error) RPCError     { return RPCError{e, rpcErrorCodeUnauthorized} }
func NewAuthRequiredError(e error) RPCError     { return RPCError{e, rpcErrorCodeAuthRequired} }

func isJSONParseError(err error) bool {
	var e RPCError
	return err != nil && errors.As(err, &e) && e.code == rpcErrorCodeJSONParse
}
