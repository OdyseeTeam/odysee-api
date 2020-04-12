package proxy

import (
	"io/ioutil"
	"net/http"

	"github.com/lbryio/lbrytv/app/users"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/internal/responses"
)

var proxyHandlerLogger = monitor.NewModuleLogger("proxy_handlers")

// RequestHandler is a wrapper for passing proxy.Service instance to proxy HTTP handler.
type RequestHandler struct {
	*Service
}

// NewRequestHandler initializes request handler with a provided Proxy Service instance
func NewRequestHandler(svc *Service) *RequestHandler {
	return &RequestHandler{Service: svc}
}

// Handle forwards client JSON-RPC request to proxy.
func (rh *RequestHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("empty request body"))
		proxyHandlerLogger.Log().Errorf("empty request body")
		return
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error reading request body"))
		proxyHandlerLogger.Log().Errorf("error reading request body: %v", err.Error())
		return
	}

	var walletID string

	q, err := NewQuery(body)
	if err != nil || !methodInList(q.Method(), relaxedMethods) {
		retriever := users.NewWalletService(rh.SDKRouter)
		auth := users.NewAuthenticator(retriever)

		walletID, err = auth.GetWalletID(r)
		if err != nil {
			responses.JSONRPCError(w, err.Error(), ErrAuthFailed)
			monitor.CaptureRequestError(err, r, w)
			return
		}
	}

	c := rh.NewCaller(walletID)

	rawCallReponse := c.Call(body)
	responses.PrepareJSONWriter(w)
	w.Write(rawCallReponse)
}

// HandleOptions returns necessary CORS headers for pre-flight requests to proxy API
func (rh *RequestHandler) HandleOptions(w http.ResponseWriter, r *http.Request) {
	hs := w.Header()
	hs.Set("Access-Control-Max-Age", "7200")
	hs.Set("Access-Control-Allow-Origin", "*")
	hs.Set("Access-Control-Allow-Headers", "X-Lbry-Auth-Token, Origin, X-Requested-With, Content-Type, Accept")
	w.WriteHeader(http.StatusOK)
}
