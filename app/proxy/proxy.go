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
	"net/http"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/lbrynet"
	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/sirupsen/logrus"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"

	"github.com/ybbus/jsonrpc"
)

var logger = monitor.NewModuleLogger("proxy")

const (
	walletLoadRetries   = 3
	walletLoadRetryWait = 100 * time.Millisecond
	rpcTimeout          = 30 * time.Second
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
			HTTPClient: &http.Client{Timeout: rpcTimeout},
		}),
		endpoint: endpoint,
		userID:   userID,
	}
}

func (c *Caller) CallRaw(rawQuery []byte) []byte {
	var req jsonrpc.RPCRequest
	err := json.Unmarshal(rawQuery, &req)
	if err != nil {
		return marshalError(NewJSONParseError(err))
	}
	return c.Call(&req)
}

// Call method processes a raw query received from JSON-RPC client and forwards it to LbrynetServer.
// It returns a response that is ready to be sent back to the JSON-RPC client as is.
func (c *Caller) Call(req *jsonrpc.RPCRequest) []byte {
	r, err := c.call(req)
	if err != nil {
		monitor.CaptureException(err, map[string]string{"req": spew.Sdump(req), "response": fmt.Sprintf("%v", r)})
		logger.Log().Errorf("error calling lbrynet: %v, request: %s", err, spew.Sdump(req))
		return marshalError(err)
	}

	serialized, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		monitor.CaptureException(err)
		logger.Log().Errorf("error marshaling response: %v", err)
		return marshalError(NewInternalError(err))
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

	callMetrics := metrics.ProxyCallDurations.WithLabelValues(q.Method(), c.endpoint)
	failureMetrics := metrics.ProxyCallFailedDurations.WithLabelValues(q.Method(), c.endpoint)

	for i := 0; i < walletLoadRetries; i++ {
		start := time.Now()

		r, err = c.client.CallRaw(q.Request)

		duration = time.Since(start).Seconds()
		callMetrics.Observe(duration)

		// Generally a HTTP transport failure (connect error etc)
		if err != nil {
			logger.Log().Errorf("error sending query to %v: %v", c.endpoint, err)
			return nil, err
		}

		// This checks if LbrynetServer responded with missing wallet error and tries to reload it,
		// then repeats the request again.
		if isErrWalletNotLoaded(r) {
			time.Sleep(walletLoadRetryWait)
			// Using LBRY JSON-RPC client here for easier request/response processing
			client := ljsonrpc.NewClient(c.endpoint)
			_, err := client.WalletAdd(sdkrouter.WalletID(c.userID))
			// Alert sentry on the last failed wallet load attempt
			if err != nil && i >= walletLoadRetries-1 {
				errMsg := "gave up on manually adding a wallet: %v"
				logger.WithFields(logrus.Fields{
					"user_id":  c.userID,
					"endpoint": c.endpoint,
				}).Errorf(errMsg, err)
				monitor.CaptureException(
					fmt.Errorf(errMsg, err), map[string]string{
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

	if (r != nil && r.Error != nil) || err != nil {
		logger.WithFields(logrus.Fields{
			"method":   q.Method(),
			"params":   q.Params(),
			"endpoint": c.endpoint,
			"user_id":  c.userID,
			"duration": duration,
			"response": r.Error,
		}).Error("error from the target endpoint")
		failureMetrics.Observe(duration)
	} else {
		fields := logrus.Fields{
			"method":   q.Method(),
			"params":   q.Params(),
			"endpoint": c.endpoint,
			"user_id":  c.userID,
			"duration": duration,
		}
		if config.ShouldLogResponses() {
			fields["response"] = r
		}
		logger.WithFields(fields).Info("call processed")
	}

	return r, err
}

func marshalError(err error) []byte {
	var rpcErr RPCError
	if errors.As(err, &rpcErr) {
		return rpcErr.JSON()
	}
	return NewInternalError(err).JSON()
}

func isErrWalletNotLoaded(r *jsonrpc.RPCResponse) bool {
	return r.Error != nil && errors.Is(lbrynet.NewWalletError(0, errors.New(r.Error.Message)), lbrynet.ErrWalletNotLoaded)
}

func isErrWalletAlreadyLoaded(r *jsonrpc.RPCResponse) bool {
	return r.Error != nil && errors.Is(lbrynet.NewWalletError(0, errors.New(r.Error.Message)), lbrynet.ErrWalletAlreadyLoaded)
}
