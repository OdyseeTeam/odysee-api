package rpcerrors

import (
	"encoding/json"

	"github.com/lbryio/lbrytv/internal/errors"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/ybbus/jsonrpc"
)

var logger = monitor.NewModuleLogger("rpc_errors")

const (
	rpcErrorCodeInternal         int = -32080 // general errors that originate inside the proxy module
	rpcErrorCodeSDK              int = -32603 // otherwise-unspecified errors from the SDK
	rpcErrorCodeAuthRequired     int = -32084 // auth info is required but is not provided
	rpcErrorCodeForbidden        int = -32085 // auth info is provided but is not found in the database
	rpcErrorCodeJSONParse        int = -32700 // invalid JSON was received by the server
	rpcErrorCodeInvalidParams    int = -32602 // error in params that the client provided
	rpcErrorCodeMethodNotAllowed int = -32601 // the requested method is not allowed to be called
)

type RPCError struct {
	err  error
	code int
}

func (e RPCError) Code() int     { return e.code }
func (e RPCError) Unwrap() error { return e.err }
func (e RPCError) Error() string {
	if e.err == nil {
		return "no wrapped error"
	}
	return e.err.Error()
}

func (e RPCError) JSON() []byte {
	b, err := json.MarshalIndent(jsonrpc.RPCResponse{
		Error: &jsonrpc.RPCError{
			Code:    e.Code(),
			Message: e.Error(),
		},
		JSONRPC: "2.0",
	}, "", "  ")
	if err != nil {
		logger.Log().Errorf("rpc error to json: %v", err)
	}
	return b
}

func newRPCErr(e error, code int) RPCError { return RPCError{errors.Err(e), code} }

func NewInternalError(e error) RPCError         { return newRPCErr(e, rpcErrorCodeInternal) }
func NewJSONParseError(e error) RPCError        { return newRPCErr(e, rpcErrorCodeJSONParse) }
func NewMethodNotAllowedError(e error) RPCError { return newRPCErr(e, rpcErrorCodeMethodNotAllowed) }
func NewInvalidParamsError(e error) RPCError    { return newRPCErr(e, rpcErrorCodeInvalidParams) }
func NewSDKError(e error) RPCError              { return newRPCErr(e, rpcErrorCodeSDK) }
func NewForbiddenError(e error) RPCError        { return newRPCErr(e, rpcErrorCodeForbidden) }
func NewAuthRequiredError(e error) RPCError     { return newRPCErr(e, rpcErrorCodeAuthRequired) }

func isJSONParseError(err error) bool {
	var e RPCError
	return err != nil && errors.As(err, &e) && e.code == rpcErrorCodeJSONParse
}

func ErrorToJSON(err error) []byte {
	var rpcErr RPCError
	if errors.As(err, &rpcErr) {
		return rpcErr.JSON()
	}
	return NewInternalError(err).JSON()
}
