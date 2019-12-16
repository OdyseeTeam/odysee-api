package monitor

import (
	"io/ioutil"

	"github.com/lbryio/lbrytv/config"

	"github.com/sirupsen/logrus"
)

// ModuleLogger contains module-specific logger details.
type ModuleLogger struct {
	ModuleName string
	Logger     *logrus.Logger
	Level      logrus.Level
}

// F can be supplied to ModuleLogger's Log function for providing additional log context.
type F map[string]interface{}

// NewModuleLogger creates a new ModuleLogger instance carrying module name
// for later `Log()` calls.
func NewModuleLogger(moduleName string) ModuleLogger {
	logger := getBaseLogger()
	l := ModuleLogger{
		ModuleName: moduleName,
		Logger:     logger,
		Level:      logger.GetLevel(),
	}
	l.Logger.SetLevel(l.Level)
	return l
}

// LogF is a deprecated method, an equivalent WithFields/WithField should be used.
func (l ModuleLogger) LogF(fields F) *logrus.Entry { return l.WithFields(fields) }

// WithFields returns a new log entry containing additional info provided by fields,
// which can be called upon with a corresponding logLevel.
// Example:
//  logger.WithFields(F{"query": "..."}).Info("query error")
func (l ModuleLogger) WithFields(fields F) *logrus.Entry {
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

// WithField is a shortcut for when a single log entry field is needed.
func (l ModuleLogger) WithField(key string, value interface{}) *logrus.Entry {
	return l.WithFields(F{key: value})
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
