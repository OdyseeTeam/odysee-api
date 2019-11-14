// ProxyService/Caller/Query is a refactoring/improvement over the previous version of proxy module
// currently contained in proxy.go. The old code should be gradually removed and replaced
// by the following approach.

package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lbryio/lbrytv/app/router"

	"github.com/lbryio/lbrytv/internal/lbrynet"
	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
	"github.com/ybbus/jsonrpc"
)

type Preprocessor func(q *Query)

// ProxyService generates Caller objects and keeps execution time metrics
// for all calls proxied through those objects.
type ProxyService struct {
	Router router.SDKRouter
	logger monitor.QueryMonitor
}

// Caller patches through JSON-RPC requests from clients, doing pre/post-processing,
// account processing and validation.
type Caller struct {
	walletID     string
	query        *jsonrpc.RPCRequest
	client       jsonrpc.RPCClient
	endpoint     string
	service      *ProxyService
	preprocessor Preprocessor
	retries      int
}

// Query is a wrapper around client JSON-RPC query for easier (un)marshaling and processing.
type Query struct {
	Request    *jsonrpc.RPCRequest
	rawRequest []byte
	walletID   string
}

// NewService is the entry point to proxy module.
// Normally only one instance of ProxyService should be created per running server.
func NewService(sdkRouter router.SDKRouter) *ProxyService {
	s := ProxyService{
		Router: sdkRouter,
		logger: monitor.NewProxyLogger(),
	}
	return &s
}

// NewCaller returns an instance of Caller ready to proxy requests.
// Note that `SetWalletID` needs to be called if an authenticated user is making this call.
func (ps *ProxyService) NewCaller(walletID string) *Caller {
	endpoint := ps.Router.GetSDKServer(walletID)
	c := Caller{
		walletID: walletID,
		client:   jsonrpc.NewClient(endpoint),
		endpoint: endpoint,
		service:  ps,
	}
	return &c
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
	}
	return false
}

func (q *Query) newResponse() *jsonrpc.RPCResponse {
	var r jsonrpc.RPCResponse
	r.ID = q.Request.ID
	r.JSONRPC = q.Request.JSONRPC
	return &r
}

func (q *Query) SetWalletID(id string) {
	q.walletID = id
}

// cacheHit returns cached response or nil in case it's a miss or query shouldn't be cacheable.
func (q *Query) cacheHit() *jsonrpc.RPCResponse {
	if q.isCacheable() {
		if cached := responseCache.Retrieve(q.Method(), q.Params()); cached != nil {
			// TODO: Temporary hack to find out why the following line doesn't work
			// if mResp, ok := cResp.(map[string]interface{}); ok {
			s, _ := json.Marshal(cached)
			response := q.newResponse()
			err := json.Unmarshal(s, &response)
			if err == nil {
				monitor.LogCachedQuery(q.Method())
				return response
			}
		}
	}
	return nil
}

func (q *Query) predefinedResponse() *jsonrpc.RPCResponse {
	if q.Method() == MethodStatus {
		response := q.newResponse()
		response.Result = getStatusResponse()
		return response
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
			return NewParamsError(errors.New("account identificator required"))
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

// SetPreprocessor applies provided function to query before it's sent to the LbrynetServer.
func (c *Caller) SetPreprocessor(p Preprocessor) {
	c.preprocessor = p
}

// WalletID is an LbrynetServer wallet ID for the client this caller instance is serving.
func (c *Caller) WalletID() string {
	return c.walletID
}

func (c *Caller) marshal(r *jsonrpc.RPCResponse) ([]byte, CallError) {
	serialized, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, NewError(err)
	}
	return serialized, nil
}

func (c *Caller) marshalError(e CallError) []byte {
	serialized, err := json.MarshalIndent(e.AsRPCResponse(), "", "  ")
	if err != nil {
		return []byte(err.Error())
	}
	return serialized
}

func (c *Caller) sendQuery(q *Query) (*jsonrpc.RPCResponse, error) {
	response, err := c.client.CallRaw(q.Request)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Caller) call(rawQuery []byte) (*jsonrpc.RPCResponse, CallError) {
	q, err := NewQuery(rawQuery)
	if err != nil {
		c.service.logger.Errorf("malformed JSON from client: %s", err.Error())
		return nil, NewParseError(err)
	}

	if c.WalletID() != "" {
		q.SetWalletID(c.WalletID())
	}

	// Check for account identificator (wallet ID) for account-specific methods happens here
	if err := q.validate(); err != nil {
		return nil, err
	}

	if cachedResponse := q.cacheHit(); cachedResponse != nil {
		return cachedResponse, nil
	}
	if predefinedResponse := q.predefinedResponse(); predefinedResponse != nil {
		return predefinedResponse, nil
	}

	if c.preprocessor != nil {
		c.preprocessor(q)
	}

	queryStartTime := time.Now()
	r, err := c.sendQuery(q)
	duration := time.Since(queryStartTime).Seconds()
	if err != nil {
		return r, NewInternalError(err)
	}

	if r.Error != nil {
		metrics.ProxyCallFailedDurations.WithLabelValues(q.Method()).Observe(duration)
		c.service.logger.LogFailedQuery(q.Method(), q.Params(), r.Error)
	} else {
		metrics.ProxyCallDurations.WithLabelValues(q.Method()).Observe(duration)
		c.service.logger.LogSuccessfulQuery(q.Method(), duration, q.Params(), r)
	}

	// This checks if LbrynetServer responded with missing wallet error and tries to reload it,
	// then repeat the request again.
	// TODO: Refactor this and move somewhere else
	if r.Error != nil && c.retries == 0 {
		wErr := lbrynet.NewWalletError(0, errors.New(r.Error.Message))
		if errors.As(wErr, &lbrynet.WalletNotLoaded{}) {
			_, err := lbrynet.Client.WalletAdd(c.WalletID())
			if err == nil {
				c.retries++
				return c.call(rawQuery)
			}
		}
	}

	r, err = processResponse(q.Request, r)
	if err != nil {
		return r, NewInternalError(err)
	}

	if q.isCacheable() {
		responseCache.Save(q.Method(), q.Params(), r)
	}
	return r, nil
}

// Call method processes a raw query received from JSON-RPC client and forwards it to LbrynetServer.
// It returns a response that is ready to be sent back to the JSON-RPC client as is.
func (c *Caller) Call(rawQuery []byte) []byte {
	r, err := c.call(rawQuery)
	if err != nil {
		monitor.CaptureException(err, map[string]string{"query": string(rawQuery), "response": fmt.Sprintf("%v", r)})
		c.service.logger.Errorf("error calling lbrynet: %v, query: %s", err, rawQuery)
		return c.marshalError(err)
	}
	serialized, err := c.marshal(r)
	if err != nil {
		monitor.CaptureException(err)
		c.service.logger.Errorf("error marshaling response: %v", err)
		return c.marshalError(err)
	}
	return serialized
}
