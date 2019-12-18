package proxy

import (
	"fmt"

	"github.com/lbryio/lbrytv/internal/responses"

	"github.com/ybbus/jsonrpc"
)

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
	if methodInList(method, forbiddenMethods) {
		return responses.NewJSONRPCError(fmt.Sprintf("Forbidden method requested: %v", method), ErrMethodUnavailable)
	}

	if paramsMap, ok := params.(map[string]interface{}); ok {
		if _, ok := paramsMap[forbiddenParam]; ok {
			return responses.NewJSONRPCError(fmt.Sprintf("Forbidden parameter supplied: %v", forbiddenParam), ErrInvalidParams)
		}
	}

	if method == MethodStatus {
		var r jsonrpc.RPCResponse
		r.Result = getStatusResponse()
		return &r
	}
	return nil
}
