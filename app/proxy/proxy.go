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
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/errors"
	"github.com/lbryio/lbrytv/internal/lbrynet"
	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"

	"github.com/davecgh/go-spew/spew"
	"github.com/sirupsen/logrus"
	"github.com/ybbus/jsonrpc"
)

var logger = monitor.NewModuleLogger("proxy")

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
	userID   int
	endpoint string
}

func NewCaller(endpoint string, userID int) *Caller {
	return &Caller{
		client: jsonrpc.NewClientWithOpts(endpoint, &jsonrpc.RPCClientOpts{
			HTTPClient: &http.Client{
				Timeout: sdkrouter.RPCTimeout,
				Transport: &http.Transport{
					Dial: (&net.Dialer{
						Timeout:   120 * time.Second,
						KeepAlive: 120 * time.Second,
					}).Dial,
					TLSHandshakeTimeout:   30 * time.Second,
					ResponseHeaderTimeout: 180 * time.Second,
					ExpectContinueTimeout: 1 * time.Second,
				},
			},
		}),
		endpoint: endpoint,
		userID:   userID,
	}
}

func (c *Caller) CallRaw(rawQuery []byte) []byte {
	var req jsonrpc.RPCRequest
	err := json.Unmarshal(rawQuery, &req)
	if err != nil {
		return errorToJSON(NewJSONParseError(err))
	}
	return c.Call(&req)
}

// Call method processes a raw query received from JSON-RPC client and forwards it to LbrynetServer.
// It returns a response that is ready to be sent back to the JSON-RPC client as is.
func (c *Caller) Call(req *jsonrpc.RPCRequest) []byte {
	r, err := c.call(req)
	if err != nil {
		monitor.ErrorToSentry(err, map[string]string{"request": spew.Sdump(req), "response": fmt.Sprintf("%v", r)})
		logger.Log().Errorf("error calling lbrynet: %v, request: %s", err, spew.Sdump(req))
		return errorToJSON(err)
	}

	serialized, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		monitor.ErrorToSentry(err)
		logger.Log().Errorf("error marshaling response: %v", err)
		return errorToJSON(NewInternalError(err))
	}

	return serialized
}

func (c *Caller) call(req *jsonrpc.RPCRequest) (*jsonrpc.RPCResponse, error) {
	q, err := NewQuery(req)
	if err != nil {
		return nil, err
	}

	if c.userID != 0 {
		q.WalletID = sdkrouter.WalletID(c.userID)
	}

	// Check for auth for account-specific methods happens here
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
		globalCache.Save(q.Method(), q.Params(), r)
	}
	return r, nil
}

func (c *Caller) callQueryWithRetry(q *Query) (*jsonrpc.RPCResponse, error) {
	var (
		r        *jsonrpc.RPCResponse
		err      error
		duration float64
	)

	for i := 0; i < walletLoadRetries; i++ {
		start := time.Now()

		r, err = c.client.CallRaw(q.Request)

		duration = time.Since(start).Seconds()
		metrics.ProxyCallDurations.WithLabelValues(q.Method(), c.endpoint).Observe(duration)

		// Generally a HTTP transport failure (connect error etc)
		if err != nil {
			logger.Log().Errorf("error sending query to %v: %v", c.endpoint, err)
			return nil, errors.Err(err)
		}

		// This checks if LbrynetServer responded with missing wallet error and tries to reload it,
		// then repeats the request again.
		if isErrWalletNotLoaded(r) {
			time.Sleep(walletLoadRetryWait)
			// Using LBRY JSON-RPC client here for easier request/response processing
			err := wallet.LoadWallet(c.endpoint, c.userID)
			// Alert sentry on the last failed wallet load attempt
			if err != nil && i >= walletLoadRetries-1 {
				e := errors.Prefix("gave up manually adding wallet", err)
				logger.WithFields(logrus.Fields{
					"user_id":  c.userID,
					"endpoint": c.endpoint,
				}).Error(e)
				monitor.ErrorToSentry(e, map[string]string{
					"user_id":  fmt.Sprintf("%d", c.userID),
					"endpoint": c.endpoint,
					"retries":  fmt.Sprintf("%d", i),
				})
			}
		} else if isErrWalletAlreadyLoaded(r) {
			continue
		} else {
			break
		}
	}

	logFields := logrus.Fields{
		"method":   q.Method(),
		"params":   q.Params(),
		"endpoint": c.endpoint,
		"user_id":  c.userID,
		"duration": duration,
	}
	if err != nil || (r != nil && r.Error != nil) {
		logFields["response"] = r.Error
		logger.WithFields(logFields).Error("rpc call error")
		metrics.ProxyCallFailedDurations.WithLabelValues(q.Method(), c.endpoint).Observe(duration)
	} else {
		if config.ShouldLogResponses() {
			logFields["response"] = r
		}
		logger.WithFields(logFields).Debug("rpc call processed")
	}

	return r, err
}

func errorToJSON(err error) []byte {
	var rpcErr RPCError
	if errors.As(err, &rpcErr) {
		return rpcErr.JSON()
	}
	return NewInternalError(err).JSON()
}

func isErrWalletNotLoaded(r *jsonrpc.RPCResponse) bool {
	return r.Error != nil && errors.Is(lbrynet.NewWalletError(0, errors.Err(r.Error.Message)), lbrynet.ErrWalletNotLoaded)
}

func isErrWalletAlreadyLoaded(r *jsonrpc.RPCResponse) bool {
	return r.Error != nil && errors.Is(lbrynet.NewWalletError(0, errors.Err(r.Error.Message)), lbrynet.ErrWalletAlreadyLoaded)
}
