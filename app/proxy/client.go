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
const walletLoadRetryWait = time.Millisecond * 200

var ClientLogger = monitor.NewModuleLogger("proxy_client")

type LbrynetClient interface {
	Call(q *Query) (*jsonrpc.RPCResponse, error)
}

type Client struct {
	rpcClient jsonrpc.RPCClient
	endpoint  string
	wallet    string
	retries   int
}

func NewClient(endpoint string, wallet string, timeout time.Duration) LbrynetClient {
	return Client{
		endpoint: endpoint,
		rpcClient: jsonrpc.NewClientWithOpts(endpoint, &jsonrpc.RPCClientOpts{
			HTTPClient: &http.Client{Timeout: time.Second * timeout}}),
		wallet: wallet,
	}
}

func (c Client) Call(q *Query) (*jsonrpc.RPCResponse, error) {
	var (
		i   int
		r   *jsonrpc.RPCResponse
		err error
	)
	for i = 0; i < walletLoadRetries; i++ {
		r, err = c.call(q)

		if err != nil {
			return nil, err
		}

		// This checks if LbrynetServer responded with missing wallet error and tries to reload it,
		// then repeat the request again.
		if c.isWalletNotLoaded(r) {
			// We need to use Lbry JSON-RPC client here for easier request/response processing
			client := ljsonrpc.NewClient(c.endpoint)
			_, err := client.WalletAdd(c.wallet)
			if err != nil {
				monitor.CaptureException(
					fmt.Errorf("encountered an error adding wallet manually: %v", err), map[string]string{
						"wallet_id": c.wallet,
						"endpoint":  c.endpoint,
					})
			}
		} else {
			return r, nil
		}
	}
	if c.isWalletNotLoaded(r) {
		monitor.CaptureException(
			fmt.Errorf("couldn't manually add wallet after %v retries", i), map[string]string{
				"wallet_id": c.wallet,
				"endpoint":  c.endpoint,
			})
	}
	return r, err
}

func (c *Client) call(q *Query) (*jsonrpc.RPCResponse, error) {
	start := time.Now()
	r, err := c.rpcClient.CallRaw(q.Request)
	duration := time.Since(start).Seconds()
	metrics.ProxyCallDurations.WithLabelValues(q.Method(), c.endpoint).Observe(duration)
	if err != nil {
		return nil, err
	}

	if r.Error != nil {
		metrics.ProxyCallFailedDurations.WithLabelValues(q.Method(), c.endpoint).Observe(duration)
		Logger.LogFailedQuery(q.Method(), duration, q.Params(), r.Error)
	} else {
		Logger.LogSuccessfulQuery(q.Method(), duration, q.Params(), r)
	}
	return r, err
}

func (c *Client) isWalletNotLoaded(r *jsonrpc.RPCResponse) bool {
	if r.Error != nil {
		wErr := lbrynet.NewWalletError(0, errors.New(r.Error.Message))
		if errors.As(wErr, &lbrynet.WalletNotLoaded{}) {
			return true
		}
	}
	return false
}
