package query

import (
	"fmt"
	"strings"

	"github.com/OdyseeTeam/odysee-api/internal/errors"
	"github.com/OdyseeTeam/odysee-api/internal/monitor"
	"github.com/OdyseeTeam/odysee-api/pkg/rpcerrors"

	"github.com/ybbus/jsonrpc/v2"
)

var logger = monitor.NewModuleLogger("query")

// Query is a wrapper around client JSON-RPC query for easier (un)marshaling and processing.
type Query struct {
	Request  *jsonrpc.RPCRequest
	WalletID string
}

// NewQuery initializes Query object with JSON-RPC request.
// The object is immediately usable and returns an error in case request parsing fails.
// If walletID is not empty, it will be added as a param to the query when the Caller calls it.
func NewQuery(req *jsonrpc.RPCRequest, walletID string) (*Query, error) {
	if strings.TrimSpace(req.Method) == "" {
		return nil, errors.Err("no method in request")
	}

	q := &Query{Request: req, WalletID: walletID}

	if !methodInList(q.Method(), relaxedMethods) && !methodInList(q.Method(), walletSpecificMethods) {
		return nil, rpcerrors.NewMethodNotAllowedError(errors.Err("forbidden method"))
	}

	if q.ParamsAsMap() != nil {
		for _, p := range forbiddenParams {
			if _, ok := q.ParamsAsMap()[p]; ok {
				return nil, rpcerrors.NewInvalidParamsError(fmt.Errorf("forbidden parameter supplied: %v", p))
			}
		}
	}

	if MethodAcceptsWallet(q.Method()) {
		if q.IsAuthenticated() {
			if p := q.ParamsAsMap(); p != nil {
				p[ParamWalletID] = q.WalletID
				q.Request.Params = p
			} else {
				q.Request.Params = map[string]interface{}{ParamWalletID: q.WalletID}
			}
		} else if MethodRequiresWallet(q.Method(), q.Params()) {
			return nil, rpcerrors.NewAuthRequiredError()
		}
	}

	return q, nil
}

// Method is a shortcut for query method.
func (q *Query) Method() string {
	return q.Request.Method
}

// Params is a shortcut for query params.
func (q *Query) Params() interface{} {
	return q.Request.Params
}

// IsAuthenticated returns true if query is performed by an authenticated user
func (q *Query) IsAuthenticated() bool {
	return q.WalletID != ""
}

// ParamsAsMap returns query params converted to a plain map.
// Warning: will not copy the map so not concurrency-friendly.
func (q *Query) ParamsAsMap() map[string]interface{} {
	if paramsMap, ok := q.Params().(map[string]interface{}); ok {
		return paramsMap
	}
	return nil
}

// CopyParamsAsMap returns a shallow copy of query params converted to a plain map.
func (q *Query) CopyParamsAsMap() map[string]interface{} {
	if paramsMap, ok := q.Params().(map[string]interface{}); ok {
		paramsCopy := map[string]interface{}{}
		for k, v := range paramsMap {
			paramsCopy[k] = v
		}
		return paramsCopy
	}
	return nil
}

func (q *Query) newResponse() *jsonrpc.RPCResponse {
	return &jsonrpc.RPCResponse{
		JSONRPC: q.Request.JSONRPC,
		ID:      q.Request.ID,
	}
}

// MethodRequiresWallet returns true for methods that require wallet_id
func MethodRequiresWallet(method string, params interface{}) bool {
	return !methodInList(method, relaxedMethods)
}

// MethodAcceptsWallet returns true for methods that can accept wallet_id
func MethodAcceptsWallet(method string) bool {
	return methodInList(method, walletSpecificMethods)
}

func methodInList(method string, checkMethods []string) bool {
	for _, m := range checkMethods {
		if m == method {
			return true
		}
	}
	return false
}
