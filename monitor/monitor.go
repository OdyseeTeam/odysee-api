package monitor

import (
	"math"
	"net/http"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/version"

	"github.com/sirupsen/logrus"
)

// Logger is a global instance of logrus object.
var Logger = logrus.New()

// TokenF is a token field name that will be stripped from logs in production mode
const TokenF = "token"

const masked = "****"

// ModuleLogger contains module-specific logger details.
type ModuleLogger struct {
	ModuleName string
	Logger     *logrus.Logger
}

// F can be supplied to ModuleLogger's Log function for providing additional log context.
type F map[string]interface{}

const responseSnippetLen = 250.

// init magic is needed so logging is set up without calling it in every package explicitly
func init() {
	SetupLogging()
}

// SetupLogging initializes and sets a few parameters for the logging subsystem.
func SetupLogging() {
	var mode string

	SetVersionTag(VersionTag{LbrytvVersion: version.GetVersion()})

	// logrus.AddHook(logrus_stack.StandardHook())
	// Logger.AddHook(logrus_stack.StandardHook())
	if config.IsProduction() {
		mode = "production"
		configureSentry(version.GetDevVersion(), mode)

		logrus.SetLevel(logrus.InfoLevel)
		Logger.SetLevel(logrus.InfoLevel)
		logrus.SetFormatter(&logrus.JSONFormatter{})
		Logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		mode = "develop"

		logrus.SetLevel(logrus.DebugLevel)
		Logger.SetLevel(logrus.DebugLevel)
		logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
		Logger.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	}

	Logger.Infof("%v, running in %v mode", version.GetFullBuildName(), mode)
	Logger.Infof("logging initialized (loglevel=%v)", Logger.Level.String())
}

// NewModuleLogger creates a new ModuleLogger instance carrying module name
// for later `Log()` calls.
func NewModuleLogger(moduleName string) ModuleLogger {
	return ModuleLogger{
		ModuleName: moduleName,
		Logger:     logrus.New(),
	}
}

// LogF returns a new log entry containing additional info provided by fields,
// which can be called upon with a corresponding logLevel.
// Example:
//  LogF("db", F{"query": "..."}).Info("query error")
func (l ModuleLogger) LogF(fields F) *logrus.Entry {
	logFields := logrus.Fields{}
	logFields["module"] = l.ModuleName
	for k, v := range fields {
		// Replace sensitive data with astericks if it's not empty to signify
		// that some value has actually been provided
		if k == TokenF && v != "" && config.IsProduction() {
			logFields[k] = masked
		} else {
			logFields[k] = v
		}
	}
	return Logger.WithFields(logFields)
}

// Log returns a new log entry for the module
// which can be called upon with a corresponding logLevel.
// Example:
//  Log().Info("query error")
func (l ModuleLogger) Log() *logrus.Entry {
	return Logger.WithFields(logrus.Fields{"module": l.ModuleName})
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
		w := &loggingWriter{ResponseWriter: writer}
		next.ServeHTTP(w, request)
		fields := logrus.Fields{
			"url":    request.URL.Path,
			"status": w.Status,
		}
		if !w.IsSuccess() {
			fields["response"] = w.ResponseSnippet
			Logger.WithFields(fields).Error("server responded with error")
		}
	})
}
