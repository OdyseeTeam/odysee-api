package handler

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/lbryio/lbrytv/internal/errors"
	"github.com/lbryio/lbrytv/internal/monitor"

	"github.com/ybbus/jsonrpc"
)

var logger = monitor.NewModuleLogger("internal_handler")

type RPCResponse struct {
	jsonrpc.RPCResponse
	Trace []string `json:"_trace,omitempty"`
}

// TraceEnabled Attaches a trace field to the JSON response when enabled.
var TraceEnabled = false

// StatusError represents an error with an associated HTTP status code.
type StatusError struct {
	Status int
	Err    error
}

func (se StatusError) Error() string { return se.Err.Error() }
func (se StatusError) Unwrap() error { return se.Err }

// Response is returned by API handlers
type Response struct {
	Status      int
	Data        interface{}
	RedirectURL string
	Error       error
}

// Handler handles API requests
type Handler func(r jsonrpc.RPCRequest) Response

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	var req jsonrpc.RPCRequest
	err = json.Unmarshal(body, &req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write(NewJSONParseError(err).JSON())
		return
	}

	logger.Log().Tracef("call to method %s", req.Method)

	rsp := h(req)

	if rsp.Status == 0 {
		if rsp.Error != nil {
			var statErr StatusError
			if errors.As(rsp.Error, &statErr) {
				rsp.Status = statErr.Status
			} else {
				rsp.Status = http.StatusInternalServerError
			}
		} else if rsp.RedirectURL != "" {
			rsp.Status = http.StatusFound
		} else {
			rsp.Status = http.StatusOK
		}
	}

	success := rsp.Status < http.StatusBadRequest
	if success {
		Log(r, &rsp, nil)
	} else {
		Log(r, &rsp, rsp.Error)
	}

	// redirect
	if rsp.Status >= http.StatusMultipleChoices && rsp.Status < http.StatusBadRequest {
		http.Redirect(w, r, rsp.RedirectURL, rsp.Status)
		return
	} else if rsp.RedirectURL != "" {
		Log(r, &rsp, errors.Base(
			"status code %d does not indicate a redirect, but RedirectURL is non-empty '%s'",
			rsp.Status, rsp.RedirectURL,
		))
	}

	var rpcError *jsonrpc.RPCError
	if rsp.Error != nil {
		rpcError = &jsonrpc.RPCError{
			Code:    -1,
			Message: rsp.Error.Error(),
		}
	}

	var trace []string
	if TraceEnabled && errors.HasTrace(rsp.Error) {
		trace = getTraceFromError(rsp.Error)
	}

	response := RPCResponse{
		RPCResponse: jsonrpc.RPCResponse{
			JSONRPC: "2.0",
			Result:  "",
			Error:   rpcError,
			ID:      1,
		},
		Trace: trace,
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		Log(r, &rsp, errors.Prefix("Error encoding JSON response: ", err))
		jsonResponse, err = json.Marshal(RPCResponse{
			RPCResponse: jsonrpc.RPCResponse{
				JSONRPC: "2.0",
				Error: &jsonrpc.RPCError{
					Code:    -2,
					Message: err.Error(),
				},
				ID: 0,
			},
			Trace: getTraceFromError(err),
		})
		if err != nil {
			Log(r, &rsp, errors.Prefix("Error encoding JSON response: ", err))
		}
	}

	w.WriteHeader(rsp.Status)
	_, _ = w.Write(jsonResponse)
}

func getTraceFromError(err error) []string {
	trace := strings.Split(errors.Trace(err), "\n")
	for index, element := range trace {
		if strings.HasPrefix(element, "\t") {
			trace[index] = strings.Replace(element, "\t", "    ", 1)
		}
	}
	return trace
}
