package router

import (
	"strconv"
	"testing"

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
	sdkRouter := New(config.GetAllLbrynets())
	assert.True(t, len(sdkRouter.GetSDKServerList()) > 0, "No servers")
}

func TestFirstServer(t *testing.T) {
	sdkRouter := New(config.GetAllLbrynets())
	server := sdkRouter.GetSDKServer("lbrytv-id.756130.wallet")
	assert.Equal(t, "http://lbrynet1:5279/", server)

	server = sdkRouter.GetSDKServer("lbrytv-id.1767731.wallet")
	assert.Equal(t, "http://lbrynet2:5279/", server)

	server = sdkRouter.GetSDKServer("lbrytv-id.751365.wallet")
	assert.Equal(t, "http://lbrynet3:5279/", server)
}

func TestServerRetrieval(t *testing.T) {
	sdkRouter := New(config.GetAllLbrynets())
	servers := sdkRouter.GetSDKServerList()
	for i := 0; i < 10000; i++ {
		iStr := strconv.Itoa(i)
		server := sdkRouter.GetSDKServer("lbrytv-id." + iStr + ".wallet")
		assert.Equal(t, servers[i%10%3].Address, server)
	}
}
