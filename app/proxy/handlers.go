package proxy

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/lbryio/lbrytv/app/users"
	"github.com/lbryio/lbrytv/internal/monitor"
)

var logger = monitor.NewModuleLogger("proxy_handlers")

// RequestHandler is a wrapper for passing proxy.ProxyService instance to proxy HTTP handler.
type RequestHandler struct {
	*ProxyService
}

// NewRequestHandler initializes request handler with a provided Proxy ProxyService instance
func NewRequestHandler(svc *ProxyService) *RequestHandler {
	return &RequestHandler{ProxyService: svc}
}

// Handle forwards client JSON-RPC request to proxy.
func (rh *RequestHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("empty request body"))
		logger.Log().Errorf("empty request body")
		return
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error reading request body"))
		logger.Log().Errorf("error reading request body: %v", err.Error())
		return
	}

	var walletID string

	q, err := NewQuery(body)
	if err != nil || !methodInList(q.Method(), relaxedMethods) {
		retriever := users.NewWalletService()
		auth := users.NewAuthenticator(retriever)
		walletID, err = auth.GetWalletID(r)

		// TODO: Refactor error response creation
		if err != nil {
			response, _ := json.Marshal(NewErrorResponse(err.Error(), ErrAuthFailed))
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write(response)
			monitor.CaptureRequestError(err, r, w)
			return
		}
	}

	c := rh.ProxyService.NewCaller(walletID)

	rawCallReponse := c.Call(body)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
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
