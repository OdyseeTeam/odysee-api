package logging

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"logur.dev/logur"
)

type ctxKey int

const loggingContextKey ctxKey = iota

var (
	loggers     = map[string]*zap.SugaredLogger{}
	environment = EnvDebug

	EnvDebug = "debug"
	EnvProd  = "prod"
)

var Prod = zap.NewProductionConfig()
var Dev = zap.NewDevelopmentConfig()

func init() {
	Prod.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	zap.ReplaceGlobals(Create("", Dev).Desugar())
}

func Create(name string, cfg zap.Config) *zap.SugaredLogger {
	l, _ := cfg.Build()
	return l.Named(name).Sugar()
}

type Logger interface {
	Debug(args ...interface{})
	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})
	Fatal(args ...interface{})
	With(keyvals ...interface{}) Logger
}

type KVLogger interface {
	Debug(msg string, keyvals ...interface{})
	Info(msg string, keyvals ...interface{})
	Warn(msg string, keyvals ...interface{})
	Error(msg string, keyvals ...interface{})
	Fatal(msg string, keyvals ...interface{})
	With(keyvals ...interface{}) KVLogger
}

type NoopKVLogger struct {
	logur.NoopKVLogger
}

type NoopLogger struct{}

func (NoopLogger) Debug(args ...interface{}) {}
func (NoopLogger) Info(args ...interface{})  {}
func (NoopLogger) Warn(args ...interface{})  {}
func (NoopLogger) Error(args ...interface{}) {}
func (NoopLogger) Fatal(args ...interface{}) {}

func (l NoopLogger) With(args ...interface{}) Logger {
	return l
}

func (l NoopKVLogger) Fatal(msg string, keyvals ...interface{}) {}

func (l NoopKVLogger) With(keyvals ...interface{}) KVLogger {
	return l
}

func AddLogRef(l KVLogger, sdHash string) KVLogger {
	if len(sdHash) >= 8 {
		return l.With("ref", sdHash[:8])
	}
	return l.With("ref?", sdHash)
}

func AddToContext(ctx context.Context, l KVLogger) context.Context {
	return context.WithValue(ctx, loggingContextKey, l)
}

func GetFromContext(ctx context.Context) KVLogger {
	l, ok := ctx.Value(loggingContextKey).(KVLogger)
	if !ok {
		return NoopKVLogger{}
	}
	return l
}
