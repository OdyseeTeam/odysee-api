package proxy

import (
	"fmt"
	"reflect"

	"github.com/lbryio/lbrytv/monitor"
	"github.com/ybbus/jsonrpc"
)

var forbiddenMethods = [...]string{
	"stop",
	"account_add",
	"account_create",
	"account_encrypt",
	"account_decrypt",
	"account_fund",
	"account_list",
	"account_lock",
	"account_remove",
	"account_set",
	"account_unlock",
	"get",
	"sync_apply",
}

const forbiddenParam = "account_id"

// getPreconditionedQueryResponse returns true if we got a resolve query with more than `cacheResolveLongerThan` urls in it
func getPreconditionedQueryResponse(method string, params interface{}) *jsonrpc.RPCResponse {
	var r *jsonrpc.RPCResponse

	for _, m := range forbiddenMethods {
		if m == method {
			return &jsonrpc.RPCResponse{Error: &jsonrpc.RPCError{Code: -32601, Message: fmt.Sprintf("Forbidden method requested: %v", method)}}
		}
	}

	monitor.Logger.Infof("WTF %v", params)
	paramsMapZ, ok := params.(map[string]interface{})
	monitor.Logger.Infof("WTF %v %v %v", reflect.TypeOf(params), paramsMapZ, ok)
	if paramsMap, ok := params.(map[string]interface{}); ok {
		monitor.Logger.Infof("FUUU %v", paramsMap[forbiddenParam])
		if paramsMap[forbiddenParam] != nil {
			return &jsonrpc.RPCResponse{Error: &jsonrpc.RPCError{Code: -32602, Message: fmt.Sprintf("Forbidden parameter supplied: %v", forbiddenParam)}}
		}
	}
	return r
}
