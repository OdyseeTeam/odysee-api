package log

import (
	"go.uber.org/zap"
)

var l, _ = zap.NewProduction()
var Log = l.Sugar().Named("watchman")
