// Package proxy handles incoming JSON-RPC requests from UI client (lbry-desktop or any other), forwards them to the actual Router instance running nearby and returns its response to the client.
// The purpose of it is to expose Router over a publicly accessible http interface,  fixing aspects of it which normally would prevent Router from being safely or efficiently shared between multiple remote clients.

// Currently it does:

// * Request validation
// * Request processing
// * Gatekeeping (blocks certain methods from being called)
// * Response processing
// * Response caching

package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/internal/monitor"

	"github.com/ybbus/jsonrpc"
)

const defaultRPCTimeout = 30 * time.Second

var Logger = monitor.NewProxyLogger()

type Preprocessor func(q *Query)

// Service generates Caller objects and keeps execution time metrics
// for all calls proxied through those objects.
type Service struct {
	SDKRouter  *sdkrouter.Router
	rpcTimeout time.Duration
	logger     monitor.QueryMonitor
}

// Caller patches through JSON-RPC requests from clients, doing pre/post-processing,
// account processing and validation.
type Caller struct {
	walletID     string
	query        *jsonrpc.RPCRequest
	client       Client
	endpoint     string
	service      *Service
	preprocessor Preprocessor
}

// NewService is the entry point to proxy module.
// Normally only one instance of Service should be created per running server.
func NewService(router *sdkrouter.Router) *Service {
	return &Service{
		SDKRouter:  router,
		rpcTimeout: defaultRPCTimeout,
	}
}

func (ps *Service) SetRPCTimeout(timeout time.Duration) {
	ps.rpcTimeout = timeout
}

// NewCaller returns an instance of Caller ready to proxy requests.
// Note that `SetWalletID` needs to be called if an authenticated user is making this call.
func (ps *Service) NewCaller(walletID string) *Caller {
	endpoint := ps.SDKRouter.GetServer(sdkrouter.UserID(walletID)).Address
	return &Caller{
		walletID: walletID,
		client:   NewClient(endpoint, walletID, ps.rpcTimeout),
		endpoint: endpoint,
		service:  ps,
	}
}

// Call method processes a raw query received from JSON-RPC client and forwards it to LbrynetServer.
// It returns a response that is ready to be sent back to the JSON-RPC client as is.
func (c *Caller) Call(rawQuery []byte) []byte {
	r, err := c.call(rawQuery)
	if err != nil {
		if !errors.As(err, &InputError{}) {
			monitor.CaptureException(err, map[string]string{"query": string(rawQuery), "response": fmt.Sprintf("%v", r)})
			Logger.Errorf("error calling lbrynet: %v, query: %s", err, rawQuery)
		}
		return c.marshalError(err)
	}
	serialized, err := c.marshal(r)
	if err != nil {
		monitor.CaptureException(err)
		Logger.Errorf("error marshaling response: %v", err)
		return c.marshalError(err)
	}
	return serialized
}

func (c *Caller) call(rawQuery []byte) (*jsonrpc.RPCResponse, CallError) {
	q, err := NewQuery(rawQuery)
	if err != nil {
		return nil, NewInputError(err)
	}

	if c.walletID != "" {
		q.SetWalletID(c.walletID)
	}

	// Check for account identifier (wallet ID) for account-specific methods happens here
	if err := q.validate(); err != nil {
		return nil, err
	}

	if cached := q.cacheHit(); cached != nil {
		return cached, nil
	}
	if pr := q.predefinedResponse(); pr != nil {
		return pr, nil
	}

	if c.preprocessor != nil {
		c.preprocessor(q)
	}

	r, err := c.client.Call(q)
	if err != nil {
		return r, NewInternalError(err)
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

// SetPreprocessor applies provided function to query before it's sent to the LbrynetServer.
func (c *Caller) SetPreprocessor(p Preprocessor) {
	c.preprocessor = p
}
