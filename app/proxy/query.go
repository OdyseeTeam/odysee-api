package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/lbryio/lbrytv/internal/responses"
	"github.com/sirupsen/logrus"

	"github.com/ybbus/jsonrpc"
)

// Query is a wrapper around client JSON-RPC query for easier (un)marshaling and processing.
type Query struct {
	Request  *jsonrpc.RPCRequest
	WalletID string
}

// NewQuery initializes Query object with JSON-RPC request supplied as bytes.
// The object is immediately usable and returns an error in case request parsing fails.
func NewQuery(req *jsonrpc.RPCRequest) (*Query, error) {
	if strings.TrimSpace(req.Method) == "" {
		return nil, errors.New("no method in request")
	}

	return &Query{Request: req}, nil
}

func (q *Query) validate() error {
	if !methodInList(q.Method(), relaxedMethods) && !methodInList(q.Method(), walletSpecificMethods) {
		return NewMethodNotAllowedError(errors.New("forbidden method"))
	}

	if q.ParamsAsMap() != nil {
		if _, ok := q.ParamsAsMap()[forbiddenParam]; ok {
			return NewInvalidParamsError(fmt.Errorf("forbidden parameter supplied: %v", forbiddenParam))
		}
	}

	if MethodNeedsAuth(q.Method()) {
		if q.WalletID == "" {
			return NewAuthRequiredError(errors.New(responses.AuthRequiredErrorMessage))
		}
		if p := q.ParamsAsMap(); p != nil {
			p[paramWalletID] = q.WalletID
			q.Request.Params = p
		} else {
			q.Request.Params = map[string]interface{}{paramWalletID: q.WalletID}
		}
	}

	return nil
}

// Method is a shortcut for query method.
func (q *Query) Method() string {
	return q.Request.Method
}

// Params is a shortcut for query params.
func (q *Query) Params() interface{} {
	return q.Request.Params
}

// ParamsAsMap returns query params converted to plain map.
func (q *Query) ParamsAsMap() map[string]interface{} {
	if paramsMap, ok := q.Params().(map[string]interface{}); ok {
		return paramsMap
	}
	return nil
}

// cacheHit returns true if we got a resolve query with more than `cacheResolveLongerThan` urls in it.
func (q *Query) isCacheable() bool {
	if q.Method() == MethodResolve && q.Params() != nil {
		paramsMap := q.Params().(map[string]interface{})
		if urls, ok := paramsMap[paramUrls].([]interface{}); ok {
			if len(urls) > cacheResolveLongerThan {
				return true
			}
		}
	} else if q.Method() == MethodClaimSearch {
		return true
	}
	return false
}

func (q *Query) newResponse() *jsonrpc.RPCResponse {
	return &jsonrpc.RPCResponse{
		JSONRPC: q.Request.JSONRPC,
		ID:      q.Request.ID,
	}
}

// cacheHit returns cached response or nil in case it's a miss or query shouldn't be cacheable.
func (q *Query) cacheHit() *jsonrpc.RPCResponse {
	if !q.isCacheable() {
		return nil
	}

	cached := globalCache.Retrieve(q.Method(), q.Params())
	if cached == nil {
		return nil
	}

	s, err := json.Marshal(cached)
	if err != nil {
		logger.Log().Errorf("error marshalling cached response")
		return nil
	}

	response := q.newResponse()
	err = json.Unmarshal(s, &response)
	if err != nil {
		return nil
	}

	logger.WithFields(logrus.Fields{"method": q.Method()}).Debug("cached query")
	return response
}

func (q *Query) predefinedResponse() *jsonrpc.RPCResponse {
	switch q.Method() {
	case MethodStatus:
		response := q.newResponse()
		response.Result = getStatusResponse()
		return response
	default:
		return nil
	}
}

func MethodNeedsAuth(method string) bool {
	return !methodInList(method, relaxedMethods)
}

func methodInList(method string, checkMethods []string) bool {
	for _, m := range checkMethods {
		if m == method {
			return true
		}
	}
	return false
}
