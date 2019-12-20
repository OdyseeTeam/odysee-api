package player

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/lbryio/lbrytv/app/users"
	"github.com/lbryio/lbrytv/internal/monitor"
)

var viewableTypes = []string{
	"audio/",
	"video/",
	"image/",
	"text/markdown",
}

const ParamDownload = "download"

// RequestHandler is a HTTP request handler for player package.
type RequestHandler struct {
	player *Player
}

// NewRequestHandler initializes a HTTP request handler with the provided Player instance.
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
	if errors.Is(err, errPaidStream) {
		h.writeErrorResponse(w, http.StatusPaymentRequired, err.Error())
	} else if errors.Is(err, errStreamNotFound) {
		h.writeErrorResponse(w, http.StatusNotFound, err.Error())
	} else if strings.Contains(err.Error(), "blob not found") {
		h.writeErrorResponse(w, http.StatusServiceUnavailable, err.Error())
	} else if strings.Contains(err.Error(), "hash in response does not match") {
		h.writeErrorResponse(w, http.StatusServiceUnavailable, err.Error())
	} else {
		monitor.CaptureException(err, map[string]string{"uri": uri})
		h.writeErrorResponse(w, http.StatusInternalServerError, err.Error())
	}
}

func (h *RequestHandler) isViewable(mime string) bool {
	for _, t := range viewableTypes {
		if strings.HasPrefix(mime, t) {
			return true
		}
	}
	return false
}

// Handle is responsible for all HTTP media delivery via player module.
func (h *RequestHandler) Handle(w http.ResponseWriter, r *http.Request) {
	uri := h.getURI(r)
	Logger.streamPlaybackRequested(uri, users.GetIPAddressForRequest(r))

	s, err := h.player.ResolveStream(uri)
	if err != nil {
		Logger.streamResolveFailed(uri, err)
		h.processStreamError(w, uri, err)
		return
	}
	Logger.streamResolved(s)

	err = h.player.RetrieveStream(s)
	if err != nil {
		Logger.streamRetrievalFailed(uri, err)
		h.processStreamError(w, uri, err)
		return
	}
	Logger.streamRetrieved(s)

	if !h.isViewable(s.ContentType) || r.URL.Query().Get(ParamDownload) != "" {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%v", s.Claim.Value.GetStream().Source.Name))
	}

	err = h.player.Play(s, w, r)
	if err != nil {
		h.processStreamError(w, uri, err)
		return
	}
}

// HandleHead handlers OPTIONS requests for media.
func (h *RequestHandler) HandleHead(w http.ResponseWriter, r *http.Request) {
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
