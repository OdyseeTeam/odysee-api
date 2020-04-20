package monitor

import (
	"io/ioutil"

	"github.com/lbryio/lbrytv/config"

	"github.com/sirupsen/logrus"
)

// ModuleLogger contains module-specific logger details.
type ModuleLogger struct {
	Logger     *logrus.Logger
	moduleName string
}

// NewModuleLogger creates a new ModuleLogger instance carrying module name
// for later `Log()` calls.
func NewModuleLogger(moduleName string) ModuleLogger {
	logger := logrus.New()
	configureLogger(logger)
	return ModuleLogger{
		moduleName: moduleName,
		Logger:     logger,
	}
}

// WithFields returns a new log entry containing additional info provided by fields,
// which can be called upon with a corresponding logLevel.
// Example:
//  logger.WithFields(F{"query": "..."}).Info("query error")
func (l ModuleLogger) WithFields(fields logrus.Fields) *logrus.Entry {
	fields["module"] = l.moduleName

	if v, ok := fields[TokenF]; ok && v != "" && config.IsProduction() {
		fields[TokenF] = ValueMask
	}
	return l.Logger.WithFields(fields)
}

// Log returns a new log entry for the module
// which can be called upon with a corresponding logLevel.
// Example:
//  Log().Info("query error")
func (l ModuleLogger) Log() *logrus.Entry {
	return l.Logger.WithFields(logrus.Fields{"module": l.moduleName})
}

// Disable turns off logging output for this module logger
func (l ModuleLogger) Disable() {
	l.Logger.SetLevel(logrus.PanicLevel)
	l.Logger.SetOutput(ioutil.Discard)
}
