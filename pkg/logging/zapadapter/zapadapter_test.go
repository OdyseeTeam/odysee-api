package zapadapter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestLogger_LogLevels(t *testing.T) {
	observedLogs, logs := observer.New(zap.InfoLevel)
	logger := New(zap.New(observedLogs))

	logger.Trace("trace message")
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	assert.Equal(t, 3, logs.Len())

	testCases := []struct {
		level   zapcore.Level
		message string
	}{
		{zap.InfoLevel, "info message"},
		{zap.WarnLevel, "warn message"},
		{zap.ErrorLevel, "error message"},
	}

	for i, tc := range testCases {
		logEntry := logs.All()[i]
		assert.Equal(t, tc.level, logEntry.Level)
		assert.Equal(t, tc.message, logEntry.Message)
	}
}

func TestLogger_With(t *testing.T) {
	observedLogs, logs := observer.New(zap.InfoLevel)
	logger := New(zap.New(observedLogs)).With("key", "value")

	logger.Info("message")

	assert.Equal(t, 1, logs.Len())

	logEntry := logs.All()[0]
	assert.Equal(t, "value", logEntry.ContextMap()["key"])
	assert.Equal(t, "message", logEntry.Message)
}

func TestNewNamedKV(t *testing.T) {
	logger := NewNamedKV("testlogger", NewLoggingOpts("info", "json"))
	logger.Info("message")

	logger = NewNamedKV("testlogger", NewLoggingOpts("debug", "console"))
	logger.Info("message")

	assert.Panics(t, func() {
		NewNamedKV("testlogger", NewLoggingOpts("", ""))
	})
}
