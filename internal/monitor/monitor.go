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
	configureLogger(logrus.StandardLogger())
}

// configureLogger sets a few parameters for the logging subsystem.
func configureLogger(l *logrus.Logger) {
	var mode string

	if config.IsProduction() {
		mode = "production"

		l.SetLevel(logrus.InfoLevel)
		l.SetFormatter(&jsonFormatter)
	} else {
		mode = "develop"

		l.SetLevel(logrus.TraceLevel)
		l.SetFormatter(&textFormatter)
	}

	l.Infof("%s, running in %s mode", version.GetFullBuildName(), mode)
	l.Infof("logging initialized (loglevel=%s)", l.Level)

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
