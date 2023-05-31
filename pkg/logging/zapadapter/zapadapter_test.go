package zapadapter

import (
	stdlog "log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestKVLogger_Write(t *testing.T) {
	core, recordedLogs := observer.New(zapcore.InfoLevel)
	zapLogger := zap.New(core)
	kvLogger := NewKV(zapLogger)
	stdLogger := stdlog.New(kvLogger, "", 0)
	logMessage := "This is a log message from the standard logger"
	structLogMessage := `event="ChunkWriteComplete" id="8a254f1b8061c2f74a76f6aa8ef59d8e" bytesWritten="26214400"`
	stdLogger.Output(2, logMessage)
	stdLogger.Output(2, structLogMessage)

	require.Equal(t, 2, recordedLogs.Len())

	logEntry := recordedLogs.All()[0]
	assert.Equal(t, logMessage, logEntry.Message)
	structLogEntry := recordedLogs.All()[1]
	assert.Equal(t, "ChunkWriteComplete", structLogEntry.Message)
	assert.Equal(t, "8a254f1b8061c2f74a76f6aa8ef59d8e", structLogEntry.ContextMap()["id"])
	assert.Equal(t, "26214400", structLogEntry.ContextMap()["bytesWritten"])
}
