package monitor

import (
	"io/ioutil"
	"os"

	"github.com/sirupsen/logrus"
)

// TODO: we could drop the custom struct completely. it doesnt add anything anymore
// ModuleLogger contains module-specific logger details.
type ModuleLogger struct {
	Entry *logrus.Entry
}

// NewModuleLogger creates a new ModuleLogger instance carrying module name
// for later `Log()` calls.
func NewModuleLogger(moduleName string) ModuleLogger {
	l := logrus.New()
	configureLogLevelAndFormat(l)
	fields := logrus.Fields{
		"module": moduleName,
	}
	hostname := os.Getenv("HOSTNAME")
	if hostname != "" {
		fields["host"] = hostname
	}
	return ModuleLogger{
		Entry: l.WithFields(fields),
	}
}

// WithFields returns a new log entry containing additional info provided by fields,
// which can be called upon with a corresponding logLevel.
// Example:
//  logger.WithFields(F{"query": "..."}).Info("query error")
func (m ModuleLogger) WithFields(fields logrus.Fields) *logrus.Entry {
	if v, ok := fields[TokenF]; ok && v != "" && isProduction() {
		fields[TokenF] = valueMask
	}
	return m.Entry.WithFields(fields)
}

// Log returns a new log entry for the module
// which can be called upon with a corresponding logLevel.
// Example:
//  Log().Info("query error")
func (m ModuleLogger) Log() *logrus.Entry {
	return m.Entry.WithFields(nil)
}

// Disable turns off logging output for this module logger
func (m ModuleLogger) Disable() {
	m.Entry.Logger.SetLevel(logrus.PanicLevel)
	m.Entry.Logger.SetOutput(ioutil.Discard)
}
