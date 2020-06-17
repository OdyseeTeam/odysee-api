package monitor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/errors"
	"github.com/lbryio/lbrytv/internal/responses"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"github.com/ybbus/jsonrpc"
)

var httpLogger = NewModuleLogger("http_monitor")

type responseRecorder struct {
	StatusCode int
	Body       *bytes.Buffer

	headerMap   http.Header
	wroteHeader bool
}

func (rr *responseRecorder) Header() http.Header {
	if rr.headerMap == nil {
		rr.headerMap = make(http.Header)
	}
	return rr.headerMap
}

func (rr *responseRecorder) Write(buf []byte) (int, error) {
	rr.WriteHeader(200)
	if rr.Body == nil {
		rr.Body = new(bytes.Buffer)
	}
	rr.Body.Write(buf)
	return len(buf), nil
}

func (rr *responseRecorder) WriteHeader(code int) {
	if rr.wroteHeader {
		return
	}
	rr.StatusCode = code
	rr.wroteHeader = true
}

func (rr *responseRecorder) send(w http.ResponseWriter) {
	// Set headers
	h := w.Header()
	for k, vals := range rr.Header() {
		for _, v := range vals {
			h.Add(k, v)
		}
	}

	// Write status code and other headers
	if rr.StatusCode > 0 {
		w.WriteHeader(rr.StatusCode)
	}

	// Write body (and headers, if they were not written above)
	if rr.Body != nil {
		w.Write(rr.Body.Bytes())
	}
}

// ErrorLoggingMiddleware intercepts panics and regular error responses from http handlers,
// handles them in a graceful way, logs and sends them to Sentry
func ErrorLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub := sentry.CurrentHub().Clone()
		hub.Scope().SetRequest(sentry.Request{}.FromHTTPRequest(r))
		ctx := sentry.SetHubOnContext(r.Context(), hub)

		// Record response from next handler, recovering any panics therein
		recorder := &responseRecorder{}
		recoveredErr := func() (err error) {
			defer errors.Recover(&err)
			next.ServeHTTP(recorder, r.WithContext(ctx))
			return err
		}()

		// No panics. Send recorded response to the real writer
		if recoveredErr == nil {
			//if recorder.StatusCode >= http.StatusBadRequest {
			//	recordRequestError(r, recorder)
			//}
			recorder.send(w)
			return
		}

		// There was a panic. Handle it and send error response

		recordPanic(recoveredErr, r, recorder)

		stack := ""
		if !config.IsProduction() {
			stack = errors.Trace(recoveredErr)
		}

		responses.AddJSONContentType(w)
		rsp, _ := json.Marshal(jsonrpc.RPCResponse{
			JSONRPC: "2.0",
			Error: &jsonrpc.RPCError{
				Code:    -1,
				Message: recoveredErr.Error(),
				Data:    stack,
			},
		})
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(rsp)
	})
}

func recordRequestError(r *http.Request, rec *responseRecorder) {
	snippetLen := len(rec.Body.String())
	if snippetLen > 500 {
		snippetLen = 500
	}

	err := errors.Err("handler responded with an error")
	httpLogger.WithFields(logrus.Fields{
		"method":   r.Method,
		"url":      r.URL.Path,
		"status":   rec.StatusCode,
		"response": rec.Body.String()[:snippetLen],
	}).Error(err)

	ErrorToSentry(err, map[string]string{
		"method":   r.Method,
		"url":      r.URL.Path,
		"status":   fmt.Sprintf("%d", rec.StatusCode),
		"response": rec.Body.String()[:snippetLen],
	})
}

func recordPanic(err error, r *http.Request, rec *responseRecorder) {
	snippetLen := len(rec.Body.String())
	if snippetLen > 500 {
		snippetLen = 500
	}

	httpLogger.WithFields(logrus.Fields{
		"method":   r.Method,
		"url":      r.URL.Path,
		"status":   rec.StatusCode,
		"response": rec.Body.String()[:snippetLen],
	}).Error(fmt.Errorf("RECOVERED PANIC: %v, trace: %s", err, errors.Trace(err)))

	hub := sentry.GetHubFromContext(r.Context())
	if hub == nil {
		hub = sentry.CurrentHub()
	}
	hub.Recover(err)
}
