package query

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/sdkrouter"
	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/errors"
	"github.com/OdyseeTeam/odysee-api/internal/lbrynet"
	"github.com/OdyseeTeam/odysee-api/internal/metrics"
	"github.com/OdyseeTeam/odysee-api/internal/monitor"
	"github.com/OdyseeTeam/odysee-api/pkg/rpcerrors"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
	"github.com/sirupsen/logrus"
	"github.com/ybbus/jsonrpc/v2"
)

const (
	walletLoadRetries   = 3
	walletLoadRetryWait = 100 * time.Millisecond
	builtinHookName     = "builtin"
	defaultRPCTimeout   = 240 * time.Second

	// AllMethodsHook is used as the first argument to Add*Hook to make it apply to all methods
	AllMethodsHook = ""
)

type HTTPRequester interface {
	Do(req *http.Request) (res *http.Response, err error)
}

// Hook is a function that can be applied to certain methods during preflight or postflight phase
// using context data about the client query being performed.
// Hooks can modify both query and response, as well as perform additional queries via supplied Caller.
// If nil is returned instead of *jsonrpc.RPCResponse, original response is returned.
type Hook func(*Caller, context.Context) (*jsonrpc.RPCResponse, error)

type hookEntry struct {
	method   string
	function Hook
	name     string
}

// Caller patches through JSON-RPC requests from clients, doing pre/post-processing,
// account processing and validation.
type Caller struct {
	// Preprocessor is applied to query before it's sent to the SDK.
	Preprocessor    func(q *Query)
	preflightHooks  []hookEntry
	postflightHooks []hookEntry

	// Cache stores cacheable queries to improve performance
	Cache *QueryCache

	Duration float64

	userID          int
	endpoint        string
	backupEndpoints []string
}

func NewCaller(endpoint string, userID int) *Caller {
	caller := &Caller{
		endpoint:        endpoint,
		userID:          userID,
		backupEndpoints: []string{},
	}
	caller.addDefaultHooks()
	return caller
}

// AddPreflightHook adds query preflight hook function,
// allowing to amend the query before it gets sent to the JSON-RPC server,
// with an option to return an early response, avoiding sending the query
// to JSON-RPC server altogether.
func (c *Caller) AddPreflightHook(method string, hf Hook, name string) {
	c.preflightHooks = append(c.preflightHooks, hookEntry{method, hf, name})
	logger.Log().Debugf("added a preflight hook for method %s, %s", method, name)
}

// AddPostflightHook adds query postflight hook function,
// allowing to amend the response before it gets sent back to the client
// or to modify log entry fields.
func (c *Caller) AddPostflightHook(method string, hf Hook, name string) {
	c.postflightHooks = append(c.postflightHooks, hookEntry{method, hf, name})
	logger.Log().Debugf("added a postflight hook for method %s, %s", method, name)
}

func (c *Caller) addDefaultHooks() {
	c.AddPreflightHook(MethodStatus, getStatusResponse, builtinHookName)
	c.AddPreflightHook(MethodGet, preflightHookGet, builtinHookName)
	c.AddPreflightHook(MethodClaimSearch, preflightHookClaimSearch, builtinHookName)

	if config.GetArfleetEnabled() {
		c.AddPostflightHook(MethodClaimSearch, postClaimSearchArfleetThumbs, builtinHookName)
		c.AddPostflightHook(MethodResolve, postResolveArfleetThumbs, builtinHookName)
	}

	// This should be applied after all preflight hooks had a chance
	c.AddPreflightHook(MethodResolve, preflightCacheHook, "cache")
	c.AddPreflightHook(MethodClaimSearch, preflightCacheHook, "cache")
}

// AddBackupEndpoints can accept a list of RPC endpoints to be called
// for retries in cache routines etc.
func (c *Caller) AddBackupEndpoints(endpoints []string) {
	c.backupEndpoints = endpoints
}

func (c *Caller) RandomizeEndpoint() {
	if c.userID != 0 || len(c.backupEndpoints) == 0 {
		return
	}
	exEndpoints := []string{}
	for _, e := range c.backupEndpoints {
		if e == c.endpoint {
			continue
		}
		exEndpoints = append(exEndpoints, e)
	}
	// #nosec G404
	c.endpoint = exEndpoints[rand.IntN(len(exEndpoints))]
}

// CloneWithoutHook is for testing and debugging purposes.
func (c *Caller) CloneWithoutHook(endpoint, method, name string) *Caller {
	cc := NewCaller(endpoint, c.userID)
	for _, h := range c.postflightHooks {
		if h.method == method && h.name == name {
			continue
		}
		cc.AddPostflightHook(h.method, h.function, h.name)
	}
	for _, h := range c.preflightHooks {
		if h.method == method && h.name == name {
			continue
		}
		cc.AddPreflightHook(h.method, h.function, h.name)
	}
	return cc
}

func (c *Caller) Endpoint() string {
	return c.endpoint
}

// Call method takes JSON-RPC request through a set of hooks and forwards it to lbrynet server.
// It returns a response that is ready to be sent back to the JSON-RPC client.
func (c *Caller) Call(ctx context.Context, req *jsonrpc.RPCRequest) (*jsonrpc.RPCResponse, error) {
	origin := OriginFromContext(ctx)
	metrics.ProxyCallCounter.WithLabelValues(req.Method, c.Endpoint(), origin).Inc()
	res, err := c.call(ctx, req)
	metrics.ProxyCallDurations.WithLabelValues(req.Method, c.Endpoint(), origin).Observe(c.Duration)
	if err != nil {
		metrics.ProxyCallFailedDurations.WithLabelValues(req.Method, c.Endpoint(), origin, metrics.FailureKindNet).Observe(c.Duration)
		metrics.ProxyCallFailedCounter.WithLabelValues(req.Method, c.Endpoint(), origin, metrics.FailureKindNet).Inc()
	}
	return res, err
}

func (c *Caller) call(ctx context.Context, req *jsonrpc.RPCRequest) (*jsonrpc.RPCResponse, error) {
	if c.Endpoint() == "" {
		return nil, errors.Err("cannot call blank endpoint")
	}

	walletID := ""
	if c.userID != 0 {
		walletID = sdkrouter.WalletID(c.userID)
	}

	q, err := NewQuery(req, walletID)
	if err != nil {
		return nil, err
	}

	// Applying preflight hooks, if any one of them returns, this will be returned as response
	var res *jsonrpc.RPCResponse
	ctx = AttachQuery(ctx, q)
	for _, hook := range c.preflightHooks {
		if isMatchingHook(q.Method(), hook) {
			res, err = hook.function(c, ctx)
			if err != nil {
				return nil, rpcerrors.NewSDKError(err)
			}
			if res != nil {
				logger.Log().Debugf("got %s response from %s hook", q.Method(), hook.name)
				return res, nil
			}
		}
	}

	return c.SendQuery(ctx, q)
}

// SendQuery is where the actual RPC call happens, bypassing all hooks and retrying in case of "wallet not loaded" errors.
func (c *Caller) SendQuery(ctx context.Context, q *Query) (*jsonrpc.RPCResponse, error) {
	var (
		r   *jsonrpc.RPCResponse
		err error
	)
	op := metrics.StartOperation("sdk", "send_query")
	defer op.End()

	for i := 0; i < walletLoadRetries; i++ {
		start := time.Now()
		client := c.getRPCClient(q.Method())
		r, err = client.CallRaw(q.Request)
		c.Duration += time.Since(start).Seconds()
		logger.Log().Debugf("sent request: %s %+v (%.2fs)", q.Method(), q.Params(), c.Duration)

		// Generally a HTTP transport failure (connect error etc)
		if err != nil {
			logger.Log().Errorf("error sending query to %v: %v", c.Endpoint(), err)
			return nil, errors.Err(err)
		}

		// This checks if LbrynetServer responded with missing wallet error and tries to reload it,
		// then repeats the request again
		if isErrWalletNotLoaded(r) {
			time.Sleep(walletLoadRetryWait)
			// Using LBRY JSON-RPC client here for easier request/response processing
			err := wallet.LoadWallet(c.Endpoint(), c.userID)
			// Alert sentry on the last failed wallet load attempt
			if err != nil && i >= walletLoadRetries-1 {
				e := errors.Prefix("gave up manually adding wallet", err)
				logger.WithFields(logrus.Fields{
					"user_id":  c.userID,
					"endpoint": c.Endpoint(),
				}).Error(e)
				monitor.ErrorToSentry(e, map[string]string{
					"user_id":  fmt.Sprintf("%d", c.userID),
					"endpoint": c.Endpoint(),
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
		"endpoint": c.Endpoint(),
		"user_id":  c.userID,
		"duration": fmt.Sprintf("%.3f", c.Duration),
	}
	// Don't log query params for "sync_apply" method,
	// and also log only some entries of lists to avoid clogging
	if q.Method() != MethodSyncApply {
		paramMap := q.ParamsAsMap()
		paramCut := cutSublistsToSize(paramMap, maxListSizeLogged)
		logFields["params"] = paramCut
	}
	logEntry := logger.WithFields(logFields)

	// Applying postflight hooks
	var hookResp *jsonrpc.RPCResponse
	ctx = AttachLogEntry(ctx, logEntry)
	ctx = AttachResponse(ctx, r)
	for _, hook := range c.postflightHooks {
		if isMatchingHook(q.Method(), hook) {
			hookResp, err = hook.function(c, ctx)
			if err != nil {
				return nil, rpcerrors.NewSDKError(err)
			}
			if hookResp != nil {
				r = hookResp
			}
		}
	}

	if err != nil || (r != nil && r.Error != nil) {
		logFields["response"] = r.Error
		logEntry.Errorf("rpc call error: %v", r.Error.Message)
	} else {
		if config.ShouldLogResponses() {
			logFields["response"] = r
		}
		logEntry.Log(getLogLevel(q.Method()), "rpc call processed")
	}

	return r, err
}

func (c *Caller) newRPCClient(timeout time.Duration) jsonrpc.RPCClient {
	client := jsonrpc.NewClientWithOpts(c.Endpoint(), &jsonrpc.RPCClientOpts{
		HTTPClient: &http.Client{
			Timeout: sdkrouter.RPCTimeout + timeout,
			Transport: &http.Transport{
				Dial: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 120 * time.Second,
				}).Dial,
				ResponseHeaderTimeout: timeout * 2,
			},
		},
	})
	return client
}

func (c *Caller) getRPCTimeout(method string) time.Duration {
	t := config.GetRPCTimeout(method)
	if t != nil {
		return *t
	}
	return defaultRPCTimeout
}

func (c *Caller) getRPCClient(method string) jsonrpc.RPCClient {
	return c.newRPCClient(c.getRPCTimeout(method))
}

func getLogLevel(m string) logrus.Level {
	if methodInList(m, []string{MethodWalletBalance, MethodSyncApply}) {
		return logrus.DebugLevel
	}
	return logrus.InfoLevel
}

func isMatchingHook(m string, hook hookEntry) bool {
	return hook.method == "" || hook.method == m || strings.HasPrefix(m, hook.method)
}

func isErrWalletNotLoaded(r *jsonrpc.RPCResponse) bool {
	err := convertToError(r)
	if err == nil {
		return false
	}
	return errors.Is(err, lbrynet.ErrWalletNotLoaded)
}

func isErrWalletAlreadyLoaded(r *jsonrpc.RPCResponse) bool {
	err := convertToError(r)
	if err == nil {
		return false
	}
	return errors.Is(err, lbrynet.ErrWalletAlreadyLoaded)
}

func convertToError(r *jsonrpc.RPCResponse) error {
	if r.Error == nil {
		return nil
	}
	if d, ok := r.Error.Data.(map[string]interface{}); ok {
		_, ok := d["name"].(string)
		if !ok {
			return nil
		}
	}
	return lbrynet.NewWalletError(ljsonrpc.WrapError(r.Error))
}

// cutSublistsToSize makes a copy of a map, cutting the size of the lists inside it
// to at most num, made for declogging logs
func cutSublistsToSize(m map[string]interface{}, num int) map[string]interface{} {
	ret := make(map[string]interface{})
	for key, value := range m {
		switch value.(type) {
		case []interface{}:

			amountSkipped := len(value.([]interface{})) - num
			if amountSkipped <= 0 {
				ret[key] = value
			} else {
				cutValue := make([]interface{}, num+1)
				copy(cutValue, value.([]interface{})[0:num])

				cutValue[num] = fmt.Sprintf("... (%d skipped)", amountSkipped)
				ret[key] = cutValue
			}
		default:
			ret[key] = value
		}
	}
	return ret
}
