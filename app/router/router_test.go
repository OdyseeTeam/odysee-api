package router

import (
	"testing"

	"github.com/lbryio/lbrytv/util/wallet"

	"github.com/lbryio/lbrytv/config"
	"github.com/stretchr/testify/assert"
)

func TestGetFirstDigit(t *testing.T) {
	digit := getLastDigit("lbrytv-id.756130.wallet")
	if digit != 5 {
		t.Error("digit of number does not match")
	}
}

func TestInitializeWithYML(t *testing.T) {
	sdkRouter := New(config.GetLbrynetServers())
	assert.True(t, len(sdkRouter.GetSDKServerList()) > 0, "No servers")
}

func TestFirstServer(t *testing.T) {
	sdkRouter := New(config.GetLbrynetServers())
	server := sdkRouter.GetSDKServer("lbrytv-id.756130.wallet")
	assert.Equal(t, "http://lbrynet1:5279/", server)

	server = sdkRouter.GetSDKServer("lbrytv-id.1767731.wallet")
	assert.Equal(t, "http://lbrynet2:5279/", server)

	server = sdkRouter.GetSDKServer("lbrytv-id.751365.wallet")
	assert.Equal(t, "http://lbrynet3:5279/", server)
}

func TestServerRetrieval(t *testing.T) {
	sdkRouter := New(config.GetLbrynetServers())
	servers := sdkRouter.GetSDKServerList()
	for i := 0; i < 10000; i++ {
		walletID := wallet.MakeID(i)
		server := sdkRouter.GetSDKServer(walletID)
		assert.Equal(t, servers[i%10%3].Address, server)
	}
}

func TestDefaultLbrynetServer(t *testing.T) {
	sdkRouter := New(config.GetLbrynetServers())
	_, ok := sdkRouter.LbrynetServers["default"]
	if !ok {
		t.Error("No default lbrynet server is specified in the lbrytv.yml")
	}
}
