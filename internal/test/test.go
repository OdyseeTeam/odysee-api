package test

import (
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/config"
)

func SDKRouter() *sdkrouter.Router {
	return sdkrouter.New(config.GetLbrynetServers())
}
