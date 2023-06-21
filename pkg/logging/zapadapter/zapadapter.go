package zapadapter

import (
	"regexp"
	"strings"

	"github.com/OdyseeTeam/odysee-api/pkg/logging"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"logur.dev/logur"
)

var prodConfig = zap.NewProductionConfig()
var devConfig = zap.NewDevelopmentConfig()

var reLegacyStructLog = regexp.MustCompile(`(\w+)="([^"]+)"`)

type LoggingOpts struct {
	level  string
	format string
}

// logger is a Logur adapter for Uber's Zap.
type logger struct {
	logger *zap.SugaredLogger
	core   zapcore.Core
}

// kvLogger is a Logur adapter for Uber's Zap.
type kvLogger struct {
	zapLogger *zap.SugaredLogger
	core      zapcore.Core
}

// New returns a new Logur kvLogger.
// If zlogger is nil, a default global instance is used.
func New(zlogger *zap.Logger) *logger {
	if zlogger == nil {
		zlogger = zap.L()
	}
	zlogger = zlogger.WithOptions(zap.AddCallerSkip(1))

	return &logger{
		logger: zlogger.Sugar(),
		core:   zlogger.Core(),
	}
}

// NewKV returns a new Logur kvLogger.
// If kvLogger is nil, a default global instance is used.
func NewKV(logger *zap.Logger) *kvLogger {
	if logger == nil {
		logger = zap.L()
	}
	logger = logger.WithOptions(zap.AddCallerSkip(1))

	return &kvLogger{
		zapLogger: logger.Sugar(),
		core:      logger.Core(),
	}
}

func NewNamedKV(name string, opts logging.LoggingOpts) *kvLogger {
	var cfg zap.Config

	if opts.Level() == logging.LevelInfo {
		cfg = zap.NewProductionConfig()
	} else if opts.Level() == logging.LevelDebug {
		cfg = zap.NewDevelopmentConfig()
	}
	cfg.Encoding = opts.Format()
	l, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	return NewKV(l.Named(name))
}

func NewLoggingOpts(level, format string) LoggingOpts {
	return LoggingOpts{
		level:  level,
		format: format,
	}
}

// Trace implements the Logur logger interface.
func (l *logger) Trace(args ...interface{}) {
	// Fall back to Debug
	l.logger.Debug(args...)
}

// Debug implements the Logur logger interface.
func (l *logger) Debug(args ...interface{}) {
	if !l.core.Enabled(zap.DebugLevel) {
		return
	}
	l.logger.Debug(args...)
}

// Info implements the Logur logger interface.
func (l *logger) Info(args ...interface{}) {
	if !l.core.Enabled(zap.InfoLevel) {
		return
	}
	l.logger.Info(args...)
}

// Warn implements the Logur logger interface.
func (l *logger) Warn(args ...interface{}) {
	if !l.core.Enabled(zap.WarnLevel) {
		return
	}
	l.logger.Warn(args...)
}

// Error implements the Logur logger interface.
func (l *logger) Error(args ...interface{}) {
	if !l.core.Enabled(zap.ErrorLevel) {
		return
	}
	l.logger.Error(args...)
}

// Error implements the Logur logger interface.
func (l *logger) Fatal(args ...interface{}) {
	l.logger.Fatal(args...)
}

// ...
func (l *logger) With(keyvals ...interface{}) logging.Logger {
	newLogger := l.logger.With(keyvals...)
	return &logger{
		logger: newLogger,
		core:   newLogger.Desugar().Core(),
	}
}

// LevelEnabled implements the Logur LevelEnabler interface.
func (l *logger) LevelEnabled(level logur.Level) bool {
	switch level {
	case logur.Trace:
		return l.core.Enabled(zap.DebugLevel)
	case logur.Debug:
		return l.core.Enabled(zap.DebugLevel)
	case logur.Info:
		return l.core.Enabled(zap.InfoLevel)
	case logur.Warn:
		return l.core.Enabled(zap.WarnLevel)
	case logur.Error:
		return l.core.Enabled(zap.ErrorLevel)
	}

	return true
}

// Trace implements the Logur kvLogger interface.
func (l *kvLogger) Trace(msg string, keyvals ...interface{}) {
	// Fall back to Debug
	l.zapLogger.Debugw(msg, keyvals...)
}

// Debug implements the Logur kvLogger interface.
func (l *kvLogger) Debug(msg string, keyvals ...interface{}) {
	if !l.core.Enabled(zap.DebugLevel) {
		return
	}
	l.zapLogger.Debugw(msg, keyvals...)
}

// Info implements the Logur kvLogger interface.
func (l *kvLogger) Info(msg string, keyvals ...interface{}) {
	if !l.core.Enabled(zap.InfoLevel) {
		return
	}
	l.zapLogger.Infow(msg, keyvals...)
}

// Warn implements the Logur kvLogger interface.
func (l *kvLogger) Warn(msg string, keyvals ...interface{}) {
	if !l.core.Enabled(zap.WarnLevel) {
		return
	}
	l.zapLogger.Warnw(msg, keyvals...)
}

// Error implements the Logur kvLogger interface.
func (l *kvLogger) Error(msg string, keyvals ...interface{}) {
	if !l.core.Enabled(zap.ErrorLevel) {
		return
	}
	l.zapLogger.Errorw(msg, keyvals...)
}

// Error implements the Logur kvLogger interface.
func (l *kvLogger) Fatal(msg string, keyvals ...interface{}) {
	l.zapLogger.Fatalw(msg, keyvals...)
}

// ...
func (l *kvLogger) With(keyvals ...interface{}) logging.KVLogger {
	newLogger := l.zapLogger.With(keyvals...)
	return &kvLogger{
		zapLogger: newLogger,
		core:      newLogger.Desugar().Core(),
	}
}

// LevelEnabled implements the Logur LevelEnabler interface.
func (l *kvLogger) LevelEnabled(level logur.Level) bool {
	switch level {
	case logur.Trace:
		return l.core.Enabled(zap.DebugLevel)
	case logur.Debug:
		return l.core.Enabled(zap.DebugLevel)
	case logur.Info:
		return l.core.Enabled(zap.InfoLevel)
	case logur.Warn:
		return l.core.Enabled(zap.WarnLevel)
	case logur.Error:
		return l.core.Enabled(zap.ErrorLevel)
	}

	return true
}

// Write writes the log data to the Zap logger.
func (l *kvLogger) Write(p []byte) (n int, err error) {
	message := strings.TrimSpace(string(p))
	matches := reLegacyStructLog.FindAllStringSubmatch(message, -1)
	if len(matches) == 0 {
		l.zapLogger.Info(message)
		return len(p), nil
	}
	fields := make([]any, 0, len(matches)-1)
	for _, m := range matches[1:] {
		fields = append(fields, m[1], m[2])
	}
	event := matches[0][2]
	l.zapLogger.Infow(event, fields...)

	return len(p), nil
}

func (o LoggingOpts) Level() string {
	return o.level
}

func (o LoggingOpts) Format() string {
	return o.format
}

func init() {
	l, _ := devConfig.Build()
	zap.ReplaceGlobals(l)
}
