package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Log = zap.NewNop().Sugar().Named("watchman")

const ()

const (
	LevelDebug      = "debug"
	LevelInfo       = "info"
	EncodingJSON    = "json"
	EncodingConsole = "console"
)

var (
	levels = map[string]zapcore.Level{LevelDebug: zapcore.DebugLevel, LevelInfo: zapcore.InfoLevel}
	// encodings = map[int]string{"json": "json", EncodingText: "text"}
)

func Configure(level string, encoding string) {
	// cfg := zap.NewProductionConfig()
	cfg := zap.NewDevelopmentConfig()
	cfg.Level = zap.NewAtomicLevelAt(levels[level])
	cfg.Encoding = encoding
	l, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	Log = l.Sugar().Named("watchman")
	Log.Infow("logger configured", "level", level, "encoding", encoding)
}
