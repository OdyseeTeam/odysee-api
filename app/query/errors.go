package query

import (
	"strings"

	"github.com/ybbus/jsonrpc/v2"
)

var userInputSDKErrors = []string{
	"expected string or bytes-like object",
}

// isUserInputError checks if error is of the kind where retrying a request with the same input will result in the same error.
func isUserInputError(resp *jsonrpc.RPCResponse) bool {
	for _, m := range userInputSDKErrors {
		if strings.Contains(resp.Error.Message, m) {
			return true
		}

	}
	return false
}
