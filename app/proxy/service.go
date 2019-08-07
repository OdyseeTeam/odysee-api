package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"

	"github.com/ybbus/jsonrpc"
)

type ProxyService struct {
	*metrics.ExecTimeMetrics
}

type Caller struct {
	accountID string
	query     *jsonrpc.RPCRequest
	client    jsonrpc.RPCClient
	service   *ProxyService
}

type Query struct {
	rawRequest []byte
	Request    *jsonrpc.RPCRequest
}

func NewProxyService() *ProxyService {
	s := ProxyService{
		metrics.NewMetrics(),
	}
	return &s
}

func NewQuery(r []byte) (*Query, error) {
	q := &Query{r, &jsonrpc.RPCRequest{}}
	err := q.unmarshal()
	if err != nil {
		return nil, err
	}
	return q, nil
}

func (q *Query) unmarshal() error {
	err := json.Unmarshal(q.rawRequest, q.Request)
	fmt.Println(string(q.rawRequest))
	if err != nil {
		return err
	}
	return nil
}

func (q *Query) Method() string {
	return q.Request.Method
}

func (q *Query) Params() interface{} {
	return q.Request.Params
}

func (q *Query) ParamsAsMap() map[string]interface{} {
	if paramsMap, ok := q.Params().(map[string]interface{}); ok {
		return paramsMap
	}
	return nil
}

// cacheHit returns true if we got a resolve query with more than `cacheResolveLongerThan` urls in it.
func (q *Query) isCacheable() bool {
	if q.Method() == methodResolve && q.Params() != nil {
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

func (q *Query) attachAccountID(id string) {
	if id != "" && methodInList(q.Method(), accountSpecificMethods) {
		// monitor.Logger.WithFields(log.Fields{
		// 	"method": r.Method, "params": r.Params,
		// }).Info("got an account-specific method call")

		if p := q.ParamsAsMap(); p != nil {
			p[paramAccountID] = id
			q.Request.Params = p
		} else {
			q.Request.Params = map[string]string{"account_id": id}
		}
	}

}

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
	if q.Method() == methodStatus {
		response := q.newResponse()
		response.Result = getStatusResponse()
		return response
	}
	return nil
}

func (q *Query) validate() CallError {
	if methodInList(q.Method(), forbiddenMethods) {
		return NewMethodError(errors.New("forbidden method"))
	}

	if q.ParamsAsMap() != nil {
		if _, ok := q.ParamsAsMap()[forbiddenParam]; ok {
			return NewParamsError(fmt.Errorf("forbidden parameter supplied: %v", forbiddenParam))
		}
	}
	return nil
}

// NewCaller returns an instance of Caller ready to proxy requests with the supplied account ID
func (ps *ProxyService) NewCaller(accountID string) *Caller {
	c := Caller{
		accountID: accountID,
		client:    jsonrpc.NewClient(config.GetLbrynet()),
		service:   ps,
	}
	return &c
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
		return nil, NewParseError(err)
	}
	if err := q.validate(); err != nil {
		return nil, err
	}

	q.attachAccountID(c.accountID)

	if cachedResponse := q.cacheHit(); cachedResponse != nil {
		return cachedResponse, nil
	}
	if predefinedResponse := q.predefinedResponse(); predefinedResponse != nil {
		return predefinedResponse, nil
	}

	queryStartTime := time.Now()
	r, err := c.sendQuery(q)
	if err != nil {
		return nil, NewInternalError(err)
	}
	execTime := time.Now().Sub(queryStartTime).Seconds()
	c.service.LogExecTime(q.Method(), execTime, q.Params())

	r, err = processResponse(q.Request, r)

	if q.isCacheable() {
		responseCache.Save(q.Method(), q.Params(), r)
	}
	return r, nil
}

// Call method processes a raw query received from JSON-RPC client and forwards it to SDK.
// It returns a response that is ready to be sent back to the JSON-RPC client as is.
func (c *Caller) Call(rawQuery []byte) []byte {
	r, err := c.call(rawQuery)
	if err != nil {
		fmt.Println(err)
		return c.marshalError(err)
	}
	serialized, err := c.marshal(r)
	if err != nil {
		return c.marshalError(err)
	}
	return serialized
}
