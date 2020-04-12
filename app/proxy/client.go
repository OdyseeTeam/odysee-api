package proxy

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/lbryio/lbrytv/internal/lbrynet"
	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
	"github.com/ybbus/jsonrpc"
)

const walletLoadRetries = 3
const walletLoadRetryWait = time.Millisecond * 100

var ClientLogger = monitor.NewModuleLogger("proxy_client")

type Client struct {
	rpcClient jsonrpc.RPCClient
	endpoint  string
	walletID  string
	retries   int
}

func NewClient(endpoint string, walletID string, timeout time.Duration) Client {
	return Client{
		endpoint: endpoint,
		rpcClient: jsonrpc.NewClientWithOpts(endpoint, &jsonrpc.RPCClientOpts{
			HTTPClient: &http.Client{Timeout: timeout},
		}),
		walletID: walletID,
	}
}

func (c Client) Call(q *Query) (*jsonrpc.RPCResponse, error) {
	var (
		r        *jsonrpc.RPCResponse
		err      error
		duration float64
	)

	callMetrics := metrics.ProxyCallDurations.WithLabelValues(q.Method(), c.endpoint)
	failureMetrics := metrics.ProxyCallFailedDurations.WithLabelValues(q.Method(), c.endpoint)

	for i := 0; i < walletLoadRetries; i++ {
		start := time.Now()

		r, err = c.rpcClient.CallRaw(q.Request)

		duration = time.Since(start).Seconds()
		callMetrics.Observe(duration)

		// Generally a HTTP transport failure (connect error etc)
		if err != nil {
			ClientLogger.Log().Errorf("error sending query to %v: %v", c.endpoint, err)
			return nil, err
		}

		// This checks if LbrynetServer responded with missing wallet error and tries to reload it,
		// then repeats the request again.
		if c.isWalletNotLoaded(r) {
			time.Sleep(walletLoadRetryWait)
			// Using LBRY JSON-RPC client here for easier request/response processing
			client := ljsonrpc.NewClient(c.endpoint)
			_, err := client.WalletAdd(c.walletID)
			// Alert sentry on the last failed wallet load attempt
			if err != nil && i >= walletLoadRetries-1 {
				errMsg := "gave up on manually adding a wallet: %v"
				ClientLogger.WithFields(monitor.F{
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
		} else if c.isWalletAlreadyLoaded(r) {
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

func (c *Client) isWalletNotLoaded(r *jsonrpc.RPCResponse) bool {
	return r.Error != nil && errors.Is(lbrynet.NewWalletError(0, errors.New(r.Error.Message)), lbrynet.ErrWalletNotLoaded)
}

func (c *Client) isWalletAlreadyLoaded(r *jsonrpc.RPCResponse) bool {
	return r.Error != nil && errors.Is(lbrynet.NewWalletError(0, errors.New(r.Error.Message)), lbrynet.ErrWalletAlreadyLoaded)
}
