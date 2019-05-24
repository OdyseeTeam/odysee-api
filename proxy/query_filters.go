package proxy

import (
	"fmt"

	"github.com/ybbus/jsonrpc"
)

var forbiddenMethods = []string{
	"stop",
	"account_add",
	"account_create",
	"account_encrypt",
	"account_decrypt",
	"account_fund",
	// "account_list",
	"account_lock",
	"account_remove",
	"account_set",
	"account_unlock",
	"get",
	"sync_apply",
}

const forbiddenParam = "account_id"

func methodInList(method string, checkMethods []string) bool {
	for _, m := range checkMethods {
		if m == method {
			return true
		}
	}
	return false
}

// getPreconditionedQueryResponse returns true if we got a resolve query with more than `cacheResolveLongerThan` urls in it
func getPreconditionedQueryResponse(method string, params interface{}) *jsonrpc.RPCResponse {
	var r *jsonrpc.RPCResponse

	if methodInList(method, forbiddenMethods) {
		return NewErrorResponse(fmt.Sprintf("Forbidden method requested: %v", method), ErrMethodNotFound)
	}

	if paramsMap, ok := params.(map[string]interface{}); ok {
		if _, ok := paramsMap[forbiddenParam]; ok {
			return NewErrorResponse(fmt.Sprintf("Forbidden parameter supplied: %v", forbiddenParam), ErrInvalidParams)
		}
	}
	return r
}
