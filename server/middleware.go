package server

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/lbryio/lbrytv/internal/monitor"
)

const responseSnippetLength = 500

var errGeneric = errors.New("handler responded with an error")

var logger = monitor.NewModuleLogger("server")

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
		w.ResponseSnippet = string(snippet)
	}
	w.ResponseSize += len(p)
	return w.ResponseWriter.Write(p)
}

func (w *loggingWriter) WriteHeader(status int) {
	w.Status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingWriter) IsSuccess() bool {
	return w.Status <= http.StatusBadRequest
}

// ErrorLoggingMiddleware intercepts panics and regular error responses from http handlers,
// handles them in a graceful way, logs and sends them to Sentry
func ErrorLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		w := &loggingWriter{ResponseWriter: writer}

		defer func() {
			var err error

			r := recover()
			if r != nil {
				switch t := r.(type) {
				case string:
					err = errors.New(t)
				case error:
					err = t
				default:
					err = fmt.Errorf("unknown error: %v", r)
				}
				http.Error(w, err.Error(), http.StatusInternalServerError)
			} else if !w.IsSuccess() {
				err = errGeneric
			}

			if err != nil {
				logger.LogF(monitor.F{
					"method":   request.Method,
					"url":      request.URL.Path,
					"status":   w.Status,
					"response": w.ResponseSnippet,
				}).Error(err)

				monitor.CaptureException(
					err,
					map[string]string{
						"method":   request.Method,
						"url":      request.URL.Path,
						"status":   string(w.Status),
						"response": w.ResponseSnippet,
					},
				)
			}
		}()

		next.ServeHTTP(w, request)
	})
}
