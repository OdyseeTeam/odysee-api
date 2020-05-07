package proxy

// Package proxy handles incoming JSON-RPC requests from UI client (lbry-desktop or any other),
// forwards them to the sdk and returns its response to the client.
// The purpose of it is to expose the SDK over a publicly accessible http interface,
// fixing aspects of it which normally would prevent SDK from being shared between multiple
// remote clients.

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/lbryio/lbrytv/app/auth"
	"github.com/lbryio/lbrytv/app/query"
	"github.com/lbryio/lbrytv/app/query/cache"
	"github.com/lbryio/lbrytv/app/rpcerrors"
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/internal/errors"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/internal/responses"

	"github.com/ybbus/jsonrpc"
)

var logger = monitor.NewModuleLogger("proxy")

// Handle forwards client JSON-RPC request to proxy.
func Handle(w http.ResponseWriter, r *http.Request) {
	responses.AddJSONContentType(w)

	if r.Body == nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write(rpcerrors.NewJSONParseError(errors.Err("empty request body")).JSON())
		logger.Log().Debugf("empty request body")
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write(rpcerrors.NewJSONParseError(errors.Err("error reading request body")).JSON())
		logger.Log().Debugf("error reading request body: %v", err.Error())
		return
	}

	var req jsonrpc.RPCRequest
	err = json.Unmarshal(body, &req)
	if err != nil {
		w.Write(rpcerrors.NewJSONParseError(err).JSON())
		return
	}

	logger.Log().Tracef("call to method %s", req.Method)

	user, err := auth.FromRequest(r)
	if query.MethodRequiresWallet(req.Method) && !rpcerrors.EnsureAuthenticated(w, user, err) {
		return
	}

	var userID int
	if query.MethodAcceptsWallet(req.Method) && user != nil {
		userID = user.ID
	}

	sdkAddress := auth.SDKAddress(user)
	if sdkAddress == "" {
		rt := sdkrouter.FromRequest(r)
		sdkAddress = rt.RandomServer().Address
	}

	var qCache cache.QueryCache
	if cache.IsOnRequest(r) {
		qCache = cache.FromRequest(r)
	}
	c := query.NewCallerWithCache(sdkAddress, userID, qCache)
	w.Write(c.Call(&req))
}

// HandleCORS returns necessary CORS headers for pre-flight requests to proxy API
func HandleCORS(w http.ResponseWriter, r *http.Request) {
	hs := w.Header()
	hs.Set("Access-Control-Max-Age", "7200")
	hs.Set("Access-Control-Allow-Origin", "*")
	hs.Set("Access-Control-Allow-Headers", wallet.TokenHeader+", Origin, X-Requested-With, Content-Type, Accept")
	w.WriteHeader(http.StatusOK)
}
