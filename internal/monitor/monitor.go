package monitor

import (
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

var jsonFormatter = logrus.JSONFormatter{DisableTimestamp: true}
var textFormatter = logrus.TextFormatter{FullTimestamp: true, TimestampFormat: "15:04:05"}

// init magic is needed so logging is set up without calling it in every package explicitly
func init() {
	SetupLogging()
}

// SetupLogging initializes and sets a few parameters for the logging subsystem.
func SetupLogging() {
	var mode string

	if config.IsProduction() {
		mode = "production"

		logrus.SetLevel(logrus.InfoLevel)
		Logger.SetLevel(logrus.InfoLevel)
		logrus.SetFormatter(&jsonFormatter)
		Logger.SetFormatter(&jsonFormatter)
	} else {
		mode = "develop"

		logrus.SetLevel(logrus.TraceLevel)
		Logger.SetLevel(logrus.TraceLevel)
		logrus.SetFormatter(&textFormatter)
		Logger.SetFormatter(&textFormatter)
	}

	Logger.Infof("%v, running in %v mode", version.GetFullBuildName(), mode)
	Logger.Infof("logging initialized (loglevel=%v)", Logger.Level.String())

	configureSentry(version.GetDevVersion(), mode)
}

// LogSuccessfulQuery takes a remote method name, execution time and params and logs it
func LogSuccessfulQuery(method string, time float64, params interface{}, response interface{}) {
	fields := logrus.Fields{
		"method":   method,
		"duration": time,
		"params":   params,
	}
	if config.ShouldLogResponses() {
		fields["response"] = response
	}
	Logger.WithFields(fields).Info("call processed")
}

// LogCachedQuery logs a cache hit for a given method
func LogCachedQuery(method string) {
	Logger.WithFields(logrus.Fields{
		"method": method,
	}).Debug("cached query")
}

type QueryMonitor interface {
	LogSuccessfulQuery(method string, time float64, params interface{}, response interface{})
	LogFailedQuery(method string, params interface{}, errorResponse interface{})
	Error(message string)
	Errorf(message string, args ...interface{})
	Logger() *logrus.Logger
}

func getBaseLogger() *logrus.Logger {
	logger := logrus.New()
	if config.IsProduction() {
		logger.SetLevel(logrus.InfoLevel)
		logger.SetFormatter(&jsonFormatter)
	} else {
		logger.SetLevel(logrus.DebugLevel)
		logger.SetFormatter(&textFormatter)
	}
	return logger
}

type ProxyLogger struct {
	logger *logrus.Logger
	entry  *logrus.Entry
	Level  logrus.Level
}

func NewProxyLogger() *ProxyLogger {
	logger := getBaseLogger()

	l := ProxyLogger{
		logger: logger,
		entry:  logger.WithFields(logrus.Fields{"module": "proxy"}),
		Level:  logger.GetLevel(),
	}
	return &l
}

func (l *ProxyLogger) LogSuccessfulQuery(method string, time float64, params interface{}, response interface{}) {
	fields := logrus.Fields{
		"method":   method,
		"duration": time,
		"params":   params,
	}
	if config.ShouldLogResponses() {
		fields["response"] = response
	}
	l.entry.WithFields(fields).Info("call processed")

}

func (l *ProxyLogger) LogFailedQuery(method string, time float64, params interface{}, errorResponse interface{}) {
	l.entry.WithFields(logrus.Fields{
		"method":   method,
		"params":   params,
		"response": errorResponse,
		"duration": time,
	}).Error("error from the target endpoint")
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
