package player

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/lbryio/lbrytv/internal/monitor"
)

// RequestHandler is a wrapper for passing proxy.ProxyService instance to proxy HTTP handler.
type RequestHandler struct {
	player *Player
}

// NewRequestHandler initializes request handler with a provided Proxy ProxyService instance
func NewRequestHandler(p *Player) *RequestHandler {
	return &RequestHandler{p}
}

func (h RequestHandler) getURI(r *http.Request) string {
	vars := mux.Vars(r)
	return fmt.Sprintf("%s#%s", vars["uri"], vars["claim"])
}

func (h RequestHandler) writeErrorResponse(w http.ResponseWriter, s int, msg string) {
	w.WriteHeader(s)
	w.Write([]byte(msg))
}

func (h RequestHandler) processStreamError(w http.ResponseWriter, uri string, err error) {
	if err == errPaidStream {
		h.writeErrorResponse(w, http.StatusPaymentRequired, err.Error())
	} else if err == errStreamNotFound {
		h.writeErrorResponse(w, http.StatusNotFound, err.Error())
	} else {
		monitor.CaptureException(err, map[string]string{"uri": uri})
		h.writeErrorResponse(w, http.StatusInternalServerError, err.Error())
	}
}

func (h *RequestHandler) Handle(w http.ResponseWriter, r *http.Request) {
	uri := h.getURI(r)
	err := h.player.Play(uri, w, r)
	if err != nil {
		h.processStreamError(w, uri, err)
		return
	}
}

func (h *RequestHandler) HandleOptions(w http.ResponseWriter, r *http.Request) {
	header := w.Header()
	uri := h.getURI(r)

	s, err := h.player.ResolveStream(uri)
	if err != nil {
		h.processStreamError(w, uri, err)
		return
	}

	err = h.player.RetrieveStream(s)
	if err != nil {
		h.processStreamError(w, uri, err)
		return
	}

	header.Set("Content-Length", fmt.Sprintf("%v", s.Size))
	header.Set("Content-Type", s.ContentType)
	header.Set("Last-Modified", s.Timestamp().UTC().Format(http.TimeFormat))
	w.WriteHeader(http.StatusOK)
}
