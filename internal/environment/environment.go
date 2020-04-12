package environment

import (
	"github.com/lbryio/lbrytv/app/proxy"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/monitor"
)

type Env struct {
	*monitor.ModuleLogger
	*config.ConfigWrapper

	proxy *proxy.Service
}

func NewEnvironment(logger *monitor.ModuleLogger, config *config.ConfigWrapper, ps *proxy.Service) *Env {
	if logger == nil {
		logger = &monitor.ModuleLogger{}
	}

	return &Env{ModuleLogger: logger, ConfigWrapper: config, proxy: ps}
}

func Null() *Env {
	return NewEnvironment(nil, nil, nil)
}
