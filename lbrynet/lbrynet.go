package lbrynet

import (
	ljsonrpc "github.com/lbryio/lbry.go/extras/jsonrpc"
	"github.com/lbryio/lbrytv/config"
)

// Client is a JSON RPC client for lbrynet daemon
var Client *ljsonrpc.Client

func init() {
	Client = ljsonrpc.NewClient(config.Settings.GetString("Lbrynet"))
}
