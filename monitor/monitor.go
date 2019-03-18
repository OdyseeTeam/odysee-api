package monitor

import (
	"math"
	"net/http"

	raven "github.com/getsentry/raven-go"
	"github.com/lbryio/lbryweb.go/config"
	log "github.com/sirupsen/logrus"
)

var Logger = log.New()

const responseSnippetLen = 250.

// SetupLogging initializes and sets a few parameters for the logging subsystem
func SetupLogging() {
	dsn := config.Settings.GetString("SentryDSN")
	if dsn != "" {
		raven.SetDSN(dsn)
	}

	// log.AddHook(logrus_stack.StandardHook())
	// Logger.AddHook(logrus_stack.StandardHook())
	if config.IsProduction() {
		raven.SetEnvironment("production")

		log.SetLevel(log.InfoLevel)
		Logger.SetLevel(log.InfoLevel)
		log.SetFormatter(&log.JSONFormatter{})
		Logger.SetFormatter(&log.JSONFormatter{})
	} else {
		raven.SetEnvironment("develop")

		log.SetLevel(log.DebugLevel)
		Logger.SetLevel(log.DebugLevel)
		log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
		Logger.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	}
}

// LogSuccessfulQuery takes a remote method name and execution time and logs it
func LogSuccessfulQuery(method string, time float64) {
	Logger.WithFields(log.Fields{
		"method":          method,
		"processing_time": time,
	}).Info("processed a call")
}

// LogFailedQuery takes a method name, query params, response error object and logs it
func LogFailedQuery(method string, query interface{}, error interface{}) {
	Logger.WithFields(log.Fields{
		"method":   method,
		"query":    query,
		"response": error,
	}).Error("server responded with error")
}

// loggingWriter mimics http.ResponseWriter but stores a snippet of response, status code
// and response size for easier logging
type loggingWriter struct {
	http.ResponseWriter
	Status          int
	ResponseSnippet string
	ResponseSize    int
}

func (w *loggingWriter) Write(p []byte) (int, error) {
	w.ResponseSnippet = string(p[:int(math.Min(float64(len(p)), responseSnippetLen))])
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

func RequestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		loggingWriter := &loggingWriter{ResponseWriter: writer}
		next.ServeHTTP(loggingWriter, request)
		fields := log.Fields{
			"url":    request.URL.Path,
			"status": loggingWriter.Status,
		}
		if loggingWriter.IsSuccess() {
			Logger.WithFields(fields).Infof("responded with %v bytes", loggingWriter.ResponseSize)
		} else {
			fields["response"] = loggingWriter.ResponseSnippet
			Logger.WithFields(fields).Error("server responded with error")
		}
	})
}
