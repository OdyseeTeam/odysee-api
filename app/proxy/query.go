package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/ybbus/jsonrpc"
)

// Query is a wrapper around client JSON-RPC query for easier (un)marshaling and processing.
type Query struct {
	Request    *jsonrpc.RPCRequest
	rawRequest []byte
	walletID   string
}

// NewQuery initializes Query object with JSON-RPC request supplied as bytes.
// The object is immediately usable and returns an error in case request parsing fails.
func NewQuery(r []byte) (*Query, error) {
	q := &Query{rawRequest: r, Request: &jsonrpc.RPCRequest{}}
	err := q.unmarshal()
	if err != nil {
		return nil, err
	}
	return q, nil
}

func (q *Query) unmarshal() error {
	err := json.Unmarshal(q.rawRequest, q.Request)
	if err != nil {
		return err
	}
	if strings.TrimSpace(q.Request.Method) == "" {
		return errors.New("invalid JSON-RPC request")
	}
	return nil
}

func (q *Query) validate() CallError {
	if !methodInList(q.Method(), relaxedMethods) && !methodInList(q.Method(), walletSpecificMethods) {
		return NewMethodError(errors.New("forbidden method"))
	}

	if q.ParamsAsMap() != nil {
		if _, ok := q.ParamsAsMap()[forbiddenParam]; ok {
			return NewParamsError(fmt.Errorf("forbidden parameter supplied: %v", forbiddenParam))
		}
	}

	if !methodInList(q.Method(), relaxedMethods) {
		if q.walletID == "" {
			return NewParamsError(errors.New("account identifier required"))
		}
		if p := q.ParamsAsMap(); p != nil {
			p[paramWalletID] = q.walletID
			q.Request.Params = p
		} else {
			q.Request.Params = map[string]interface{}{paramWalletID: q.walletID}
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

// ParamsToStruct returns query params parsed into a supplied structure.
func (q *Query) ParamsToStruct(targetStruct interface{}) error {
	return ljsonrpc.Decode(q.Params(), targetStruct)
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

func (q *Query) SetWalletID(id string) {
	q.walletID = id
}

// cacheHit returns cached response or nil in case it's a miss or query shouldn't be cacheable.
func (q *Query) cacheHit() *jsonrpc.RPCResponse {
	if !q.isCacheable() {
		return nil
	}

	cached := responseCache.Retrieve(q.Method(), q.Params())
	if cached == nil {
		return nil
	}

	s, err := json.Marshal(cached)
	if err != nil {
		Logger.Errorf("error marshalling cached response")
		return nil
	}

	response := q.newResponse()
	err = json.Unmarshal(s, &response)
	if err != nil {
		return nil
	}

	monitor.LogCachedQuery(q.Method())
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
