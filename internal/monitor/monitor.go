package monitor

import (
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/version"

	"github.com/sirupsen/logrus"
)

var logger = NewModuleLogger("monitor")

const (
	// TokenF is a token field name that will be stripped from logs in production mode.
	TokenF = "token"
	// valueMask is what replaces sensitive fields contents in logs.
	valueMask = "****"
)

var jsonFormatter = logrus.JSONFormatter{DisableTimestamp: true}
var textFormatter = logrus.TextFormatter{FullTimestamp: true, TimestampFormat: "15:04:05"}

// init magic is needed so logging is set up without calling it in every package explicitly
func init() {
	l := logrus.StandardLogger()
	configureLogLevelAndFormat(l)

	l.WithFields(
		version.BuildInfo(),
	).WithFields(logrus.Fields{
		"mode":     mode(),
		"logLevel": l.Level,
	}).Infof("standard logger configured")

	configureSentry(version.GetDevVersion(), mode())
}

func mode() string {
	if config.IsProduction() {
		return "production"
	} else {
		return "develop"
	}
}

func configureLogLevelAndFormat(l *logrus.Logger) {
	if config.IsProduction() {
		l.SetLevel(logrus.InfoLevel)
		l.SetFormatter(&jsonFormatter)
	} else {
		l.SetLevel(logrus.TraceLevel)
		l.SetFormatter(&textFormatter)
	}
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
	logger.WithFields(fields).Info("call processed")
}
