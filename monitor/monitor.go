package monitor

import (
	"math"
	"net/http"

	raven "github.com/getsentry/raven-go"
	"github.com/lbryio/lbrytv/config"
	"github.com/sirupsen/logrus"
)

var Logger = logrus.New()

// F can be supplied to Log function for providing additional log context
type F map[string]interface{}

const responseSnippetLen = 250.

// SetupLogging initializes and sets a few parameters for the logging subsystem
func SetupLogging() {
	dsn := config.Settings.GetString("SentryDSN")
	if dsn != "" {
		raven.SetDSN(dsn)
	}

	// logrus.AddHook(logrus_stack.StandardHook())
	// Logger.AddHook(logrus_stack.StandardHook())
	if config.IsProduction() {
		raven.SetEnvironment("production")

		logrus.SetLevel(logrus.InfoLevel)
		Logger.SetLevel(logrus.InfoLevel)
		logrus.SetFormatter(&logrus.JSONFormatter{})
		Logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		raven.SetEnvironment("develop")

		logrus.SetLevel(logrus.DebugLevel)
		Logger.SetLevel(logrus.DebugLevel)
		logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
		Logger.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	}
}

// Log returns a new log entry containing additional info provided by module and fields.
// Example:
//  Log("db", F{"query": "..."}).Info("query error")
func Log(module string, fields F) *logrus.Entry {
	logFields := logrus.Fields{}
	logFields["module"] = module
	for k, v := range fields {
		logFields[k] = v
	}
	return Logger.WithFields(logFields)
}

// LogSuccessfulQuery takes a remote method name and execution time and logs it
func LogSuccessfulQuery(method string, time float64) {
	Logger.WithFields(logrus.Fields{
		"method":          method,
		"processing_time": time,
	}).Info("processed a call")
}

// LogCachedQuery logs a cache hit for a given method
func LogCachedQuery(method string) {
	Logger.WithFields(logrus.Fields{
		"method": method,
	}).Info("processed a cached query")
}

// LogFailedQuery takes a method name, query params, response error object and logs it
func LogFailedQuery(method string, query interface{}, error interface{}) {
	Logger.WithFields(logrus.Fields{
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
		fields := logrus.Fields{
			"url":    request.URL.Path,
			"status": loggingWriter.Status,
		}
		if !loggingWriter.IsSuccess() {
			fields["response"] = loggingWriter.ResponseSnippet
			Logger.WithFields(fields).Error("server responded with error")
		}
	})
}
