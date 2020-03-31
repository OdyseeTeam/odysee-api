package test

import (
	"github.com/lbryio/lbrytv/app/router"
	"github.com/lbryio/lbrytv/config"
)

func SDKRouter() *router.SDK {
	return router.New(config.GetLbrynetServers())
}
