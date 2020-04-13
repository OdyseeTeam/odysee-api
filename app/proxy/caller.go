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

	"github.com/lbryio/lbrytv/internal/lbrynet"
	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"

	"github.com/sirupsen/logrus"
	"github.com/ybbus/jsonrpc"
)

var Logger = monitor.NewProxyLogger()

const (
	walletLoadRetries   = 3
	walletLoadRetryWait = 100 * time.Millisecond
)

// Caller patches through JSON-RPC requests from clients, doing pre/post-processing,
// account processing and validation.
type Caller struct {
	// Preprocessor is applied to query before it's sent to the SDK.
	Preprocessor func(q *Query)

	client   jsonrpc.RPCClient
	walletID string
	endpoint string
}

func NewCaller(endpoint, walletID string) *Caller {
	return &Caller{
		client:   jsonrpc.NewClient(endpoint),
		endpoint: endpoint,
		walletID: walletID,
	}
}

// Call method processes a raw query received from JSON-RPC client and forwards it to LbrynetServer.
// It returns a response that is ready to be sent back to the JSON-RPC client as is.
func (c *Caller) Call(rawQuery []byte) []byte {
	r, err := c.call(rawQuery)
	if err != nil {
		if !isJSONParseError(err) {
			monitor.CaptureException(err, map[string]string{"query": string(rawQuery), "response": fmt.Sprintf("%v", r)})
			Logger.Errorf("error calling lbrynet: %v, query: %s", err, rawQuery)
		}
		return marshalError(err)
	}

	serialized, err := marshalResponse(r)
	if err != nil {
		monitor.CaptureException(err)
		Logger.Errorf("error marshaling response: %v", err)
		return marshalError(err)
	}

	return serialized
}

func (c *Caller) call(rawQuery []byte) (*jsonrpc.RPCResponse, error) {
	q, err := NewQuery(rawQuery)
	if err != nil {
		return nil, err
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

	if c.Preprocessor != nil {
		c.Preprocessor(q)
	}

	r, err := c.callQueryWithRetry(q)
	if err != nil {
		return r, NewSDKError(err)
	}

	err = postProcessResponse(r, q.Request)
	if err != nil {
		return r, NewSDKError(err)
	}

	if q.isCacheable() {
		responseCache.Save(q.Method(), q.Params(), r)
	}
	return r, nil
}

func (c *Caller) callQueryWithRetry(q *Query) (*jsonrpc.RPCResponse, error) {
	var (
		r        *jsonrpc.RPCResponse
		err      error
		duration float64
	)

	callMetrics := metrics.ProxyCallDurations.WithLabelValues(q.Method(), c.endpoint)
	failureMetrics := metrics.ProxyCallFailedDurations.WithLabelValues(q.Method(), c.endpoint)

	for i := 0; i < walletLoadRetries; i++ {
		start := time.Now()

		r, err = c.client.CallRaw(q.Request)

		duration = time.Since(start).Seconds()
		callMetrics.Observe(duration)

		// Generally a HTTP transport failure (connect error etc)
		if err != nil {
			Logger.Errorf("error sending query to %v: %v", c.endpoint, err)
			return nil, err
		}

		// This checks if LbrynetServer responded with missing wallet error and tries to reload it,
		// then repeats the request again.
		if isErrWalletNotLoaded(r) {
			time.Sleep(walletLoadRetryWait)
			// Using LBRY JSON-RPC client here for easier request/response processing
			client := ljsonrpc.NewClient(c.endpoint)
			_, err := client.WalletAdd(c.walletID)
			// Alert sentry on the last failed wallet load attempt
			if err != nil && i >= walletLoadRetries-1 {
				errMsg := "gave up on manually adding a wallet: %v"
				Logger.Logger().WithFields(logrus.Fields{
					"wallet_id": c.walletID,
					"endpoint":  c.endpoint,
				}).Errorf(errMsg, err)
				monitor.CaptureException(
					fmt.Errorf(errMsg, err), map[string]string{
						"wallet_id": c.walletID,
						"endpoint":  c.endpoint,
						"retries":   fmt.Sprintf("%v", i),
					})
			}
		} else if isErrWalletAlreadyLoaded(r) {
			continue
		} else {
			break
		}
	}

	if (r != nil && r.Error != nil) || err != nil {
		Logger.LogFailedQuery(q.Method(), c.endpoint, c.walletID, duration, q.Params(), r.Error)
		failureMetrics.Observe(duration)
	} else {
		Logger.LogSuccessfulQuery(q.Method(), c.endpoint, c.walletID, duration, q.Params(), r)
	}

	return r, err
}

func marshalResponse(r *jsonrpc.RPCResponse) ([]byte, error) {
	serialized, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, NewInternalError(err)
	}
	return serialized, nil
}

func marshalError(err error) []byte {
	var rpcErr RPCError
	if errors.As(err, &rpcErr) {
		return rpcErr.JSON()
	}
	return []byte(err.Error())
}

func isErrWalletNotLoaded(r *jsonrpc.RPCResponse) bool {
	return r.Error != nil && errors.Is(lbrynet.NewWalletError(0, errors.New(r.Error.Message)), lbrynet.ErrWalletNotLoaded)
}

func isErrWalletAlreadyLoaded(r *jsonrpc.RPCResponse) bool {
	return r.Error != nil && errors.Is(lbrynet.NewWalletError(0, errors.New(r.Error.Message)), lbrynet.ErrWalletAlreadyLoaded)
}
