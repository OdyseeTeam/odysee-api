package monitor

import (
	"io/ioutil"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/version"

	"github.com/sirupsen/logrus"
)

// Logger is a global instance of logrus object.
var Logger = logrus.New()

// TokenF is a token field name that will be stripped from logs in production mode.
const TokenF = "token"

// ValueMask is what replaces sensitive fields contents in logs.
const ValueMask = "****"

// ModuleLogger contains module-specific logger details.
type ModuleLogger struct {
	ModuleName string
	Logger     *logrus.Logger
}

// F can be supplied to ModuleLogger's Log function for providing additional log context.
type F map[string]interface{}

// init magic is needed so logging is set up without calling it in every package explicitly
func init() {
	SetupLogging()
}

// SetupLogging initializes and sets a few parameters for the logging subsystem.
func SetupLogging() {
	var mode string

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
//  LogF("storage", F{"query": "..."}).Info("query error")
func (l ModuleLogger) LogF(fields F) *logrus.Entry {
	logFields := logrus.Fields{}
	logFields["module"] = l.ModuleName
	for k, v := range fields {
		if k == TokenF && v != "" && config.IsProduction() {
			logFields[k] = ValueMask
		} else {
			logFields[k] = v
		}
	}
	return l.Logger.WithFields(logFields)
}

// Log returns a new log entry for the module
// which can be called upon with a corresponding logLevel.
// Example:
//  Log().Info("query error")
func (l ModuleLogger) Log() *logrus.Entry {
	return l.Logger.WithFields(logrus.Fields{"module": l.ModuleName})
}

// Disable turns off logging output for this module logger
func (l ModuleLogger) Disable() {
	l.Logger.SetLevel(logrus.PanicLevel)
	l.Logger.SetOutput(ioutil.Discard)
}

// LogSuccessfulQuery takes a remote method name, execution time and params and logs it
func LogSuccessfulQuery(method string, time float64, params interface{}) {
	Logger.WithFields(logrus.Fields{
		"method": method,
		"time":   time,
		"params": params,
	}).Info("call processed")
}

// LogCachedQuery logs a cache hit for a given method
func LogCachedQuery(method string) {
	Logger.WithFields(logrus.Fields{
		"method": method,
	}).Info("cached query")
}

// LogFailedQuery takes a method name, query params, response error object and logs it
func LogFailedQuery(method string, query interface{}, errorResponse interface{}) {
	Logger.WithFields(logrus.Fields{
		"method":   method,
		"query":    query,
		"response": errorResponse,
	}).Error("daemon responded with an error")

	captureFailedQuery(method, query, errorResponse)
}

type QueryMonitor interface {
	LogSuccessfulQuery(method string, time float64, params interface{})
	LogFailedQuery(method string, params interface{}, errorResponse interface{})
	Error(message string)
	Errorf(message string, args ...interface{})
	Logger() *logrus.Logger
}

func getBaseLogger() *logrus.Logger {
	logger := logrus.New()
	if config.IsProduction() {
		logger.SetLevel(logrus.InfoLevel)
		logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logger.SetLevel(logrus.DebugLevel)
		logger.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	}
	return logger
}

type ProxyLogger struct {
	logger *logrus.Logger
	entry  *logrus.Entry
}

func NewProxyLogger() *ProxyLogger {
	logger := getBaseLogger()

	l := ProxyLogger{
		logger: logger,
		entry:  logger.WithFields(logrus.Fields{"module": "proxy"}),
	}
	return &l
}

func (l *ProxyLogger) LogSuccessfulQuery(method string, time float64, params interface{}) {
	l.entry.WithFields(logrus.Fields{
		"method":    method,
		"exec_time": time,
		"params":    params,
	}).Info("call proxied")
}

func (l *ProxyLogger) LogFailedQuery(method string, params interface{}, errorResponse interface{}) {
	l.entry.WithFields(logrus.Fields{
		"method":   method,
		"params":   params,
		"response": errorResponse,
	}).Error("error from the target endpoint")

	captureFailedQuery(method, params, errorResponse)
}

func (l *ProxyLogger) Error(message string) {
	l.entry.Error(message)
}

func (l *ProxyLogger) Errorf(message string, args ...interface{}) {
	l.entry.Errorf(message, args...)
}

func (l *ProxyLogger) Logger() *logrus.Logger {
	return l.logger
}
