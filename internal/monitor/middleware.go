package monitor

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
)

const responseSnippetLength = 500

var errGeneric = errors.New("handler responded with an error")

var httpLogger = NewModuleLogger("http_monitor")

// loggingWriter mimics http.ResponseWriter but stores a snippet of response, status code
// and response size for easier logging
type loggingWriter struct {
	http.ResponseWriter
	Status          int
	ResponseSnippet string
	ResponseSize    int
}

func (w *loggingWriter) Write(p []byte) (int, error) {
	if w.ResponseSnippet == "" {
		var snippet []byte
		if len(p) > responseSnippetLength {
			snippet = p[:responseSnippetLength]
		} else {
			snippet = p
		}
		w.ResponseSnippet = strings.Trim(string(snippet), "\n")
	}
	w.ResponseSize += len(p)
	return w.ResponseWriter.Write(p)
}

func (w *loggingWriter) WriteHeader(status int) {
	w.Status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingWriter) IsSuccess() bool {
	return w.Status < http.StatusBadRequest
}

// ErrorLoggingMiddleware intercepts panics and regular error responses from http handlers,
// handles them in a graceful way, logs and sends them to Sentry
func ErrorLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
		hub := sentry.CurrentHub().Clone()
		hub.Scope().SetRequest(sentry.Request{}.FromHTTPRequest(r))
		ctx := sentry.SetHubOnContext(
			r.Context(),
			hub,
		)

		w := &loggingWriter{ResponseWriter: writer}

		defer func() {
			var finalErr error
			if err := recover(); err != nil {
				switch t := err.(type) {
				case string:
					finalErr = errors.New(t)
				case error:
					finalErr = t
				default:
					finalErr = fmt.Errorf("unknown error: %v", err)
				}
				http.Error(w, finalErr.Error(), http.StatusInternalServerError)
			} else if !w.IsSuccess() {
				finalErr = errGeneric
			}

			if finalErr != nil {
				CaptureRequestError(finalErr, r, w)
			}
		}()

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func CaptureRequestError(err error, r *http.Request, w http.ResponseWriter) {
	fields := logrus.Fields{
		"method": r.Method,
		"url":    r.URL.Path,
	}
	if lw, ok := w.(*loggingWriter); ok {
		fields["status"] = fmt.Sprintf("%v", lw.Status)
		fields["response"] = lw.ResponseSnippet
	}

	httpLogger.WithFields(fields).Error(err)
	CaptureException(err)
	// if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
	// 	hub.WithScope(func(scope *sentry.Scope) {
	// 		scope.SetExtras(extra)

	// 	})
	// 	hub.RecoverWithContext(
	// 		context.WithValue(r.Context(), sentry.RequestContextKey, r),
	// 		err,
	// 	)
	// }
}
