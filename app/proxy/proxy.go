package proxy

// Package proxy handles incoming JSON-RPC requests from UI client (lbry-desktop or any other),
// forwards them to the sdk and returns its response to the client.
// The purpose of it is to expose the SDK over a publicly accessible http interface,
// fixing aspects of it which normally would prevent SDK from being shared between multiple
// remote clients.

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/lbryio/lbrytv/app/auth"
	"github.com/lbryio/lbrytv/app/query"
	"github.com/lbryio/lbrytv/app/query/cache"
	"github.com/lbryio/lbrytv/app/rpcerrors"
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/internal/audit"
	"github.com/lbryio/lbrytv/internal/errors"
	"github.com/lbryio/lbrytv/internal/ip"
	"github.com/lbryio/lbrytv/internal/lbrynext"
	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/internal/responses"
	"github.com/lbryio/lbrytv/models"
	"github.com/sirupsen/logrus"

	"github.com/ybbus/jsonrpc"
)

var logger = monitor.NewModuleLogger("proxy")

// observeFailure requires metrics.MeasureMiddleware middleware to be present on the request
func observeFailure(d float64, method, kind string) {
	metrics.ProxyE2ECallDurations.WithLabelValues(method).Observe(d)
	metrics.ProxyE2ECallFailedDurations.WithLabelValues(method, kind).Observe(d)
	metrics.ProxyE2ECallCounter.WithLabelValues(method).Inc()
	metrics.ProxyE2ECallFailedCounter.WithLabelValues(method, kind).Inc()
}

// observeSuccess requires metrics.MeasureMiddleware middleware to be present on the request
func observeSuccess(d float64, method string) {
	metrics.ProxyE2ECallDurations.WithLabelValues(method).Observe(d)
	metrics.ProxyE2ECallCounter.WithLabelValues(method).Inc()
}

// Handle forwards client JSON-RPC request to proxy.
func Handle(w http.ResponseWriter, r *http.Request) {
	responses.AddJSONContentType(w)

	if r.Body == nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write(rpcerrors.NewJSONParseError(errors.Err("empty request body")).JSON())

		observeFailure(metrics.GetDuration(r), "", metrics.FailureKindClient)
		logger.Log().Debugf("empty request body")
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write(rpcerrors.NewJSONParseError(errors.Err("error reading request body")).JSON())

		observeFailure(metrics.GetDuration(r), "", metrics.FailureKindClient)
		logger.Log().Debugf("error reading request body: %v", err.Error())

		return
	}

	var rpcReq *jsonrpc.RPCRequest
	err = json.Unmarshal(body, &rpcReq)
	if err != nil {
		w.Write(rpcerrors.NewJSONParseError(err).JSON())

		observeFailure(metrics.GetDuration(r), "", metrics.FailureKindClientJSON)
		logger.Log().Debugf("error unmarshaling request body: %v", err)

		return
	}

	logger.Log().Tracef("call to method %s", rpcReq.Method)

	user, err := auth.FromRequest(r)
	if query.MethodRequiresWallet(rpcReq.Method, rpcReq.Params) {
		authErr := GetAuthError(user, err)
		if authErr != nil {
			w.Write(rpcerrors.ErrorToJSON(authErr))
			observeFailure(metrics.GetDuration(r), rpcReq.Method, metrics.FailureKindAuth)

			return
		}
	}

	var userID int
	if query.MethodAcceptsWallet(rpcReq.Method) && user != nil {
		userID = user.ID
	}

	sdkAddress := sdkrouter.GetSDKAddress(user)
	if sdkAddress == "" {
		rt := sdkrouter.FromRequest(r)
		sdkAddress = rt.RandomServer().Address
	}

	var qCache cache.QueryCache
	if cache.IsOnRequest(r) {
		qCache = cache.FromRequest(r)
	}
	c := query.NewCaller(sdkAddress, userID)

	remoteIP := ip.FromRequest(r)
	// Logging remote IP with query
	c.AddPostflightHook("wallet_", func(_ *query.Caller, hctx *query.HookContext) (*jsonrpc.RPCResponse, error) {
		hctx.AddLogField("remote_ip", remoteIP)
		return nil, nil
	}, "")
	c.AddPostflightHook(query.MethodWalletSend, func(_ *query.Caller, hctx *query.HookContext) (*jsonrpc.RPCResponse, error) {
		audit.LogQuery(userID, remoteIP, query.MethodWalletSend, body)
		return nil, nil
	}, "")

	lbrynext.InstallHooks(c)
	c.Cache = qCache

	rpcRes, err := c.Call(rpcReq)

	if err != nil {
		monitor.ErrorToSentry(err, map[string]string{"request": fmt.Sprintf("%+v", rpcReq), "response": fmt.Sprintf("%+v", rpcRes)})
		w.Write(rpcerrors.ToJSON(err))

		logger.Log().Errorf("error calling lbrynet: %v, request: %+v", err, rpcReq)
		observeFailure(metrics.GetDuration(r), rpcReq.Method, metrics.FailureKindNet)

		return
	}
	serialized, err := responses.JSONRPCSerialize(rpcRes)
	if err != nil {
		monitor.ErrorToSentry(err)

		w.Write(rpcerrors.NewInternalError(err).JSON())

		logger.Log().Errorf("error marshaling response: %v", err)
		observeFailure(metrics.GetDuration(r), rpcReq.Method, metrics.FailureKindRPCJSON)

		return
	}

	if rpcRes.Error != nil {
		observeFailure(metrics.GetDuration(r), rpcReq.Method, metrics.FailureKindRPC)
		logger.WithFields(logrus.Fields{
			"method":   rpcReq.Method,
			"endpoint": sdkAddress,
			"response": rpcRes.Error,
		}).Errorf("proxy handler got rpc error: %v", rpcRes.Error)
	} else {
		observeSuccess(metrics.GetDuration(r), rpcReq.Method)
	}

	w.Write(serialized)
}

// HandleCORS returns necessary CORS headers for pre-flight requests to proxy API
func HandleCORS(w http.ResponseWriter, r *http.Request) {
	hs := w.Header()
	hs.Set("Access-Control-Max-Age", "7200")
	hs.Set("Access-Control-Allow-Origin", "*")
	hs.Set("Access-Control-Allow-Headers", wallet.TokenHeader+", Origin, X-Requested-With, Content-Type, Accept")
	w.WriteHeader(http.StatusOK)
}

func GetAuthError(user *models.User, err error) error {
	if err == nil && user != nil {
		return nil
	}

	if errors.Is(err, auth.ErrNoAuthInfo) {
		return rpcerrors.NewAuthRequiredError()
	} else if err != nil {
		return rpcerrors.NewForbiddenError(err)
	} else if user == nil {
		return rpcerrors.NewForbiddenError(errors.Err("must authenticate"))
	}

	return errors.Err("unknown auth error")
}
