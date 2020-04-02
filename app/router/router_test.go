package router

import (
	"os"
	"testing"

	"github.com/lbryio/lbrytv/internal/storage"

	"github.com/lbryio/lbrytv/util/wallet"

	"github.com/lbryio/lbrytv/config"
	"github.com/stretchr/testify/assert"
)

func TestGetLastDigit(t *testing.T) {
	userID := getUserID("lbrytv-id.756135.wallet")
	assert.Equal(t, 5, userID%10)
}

func TestInitializeWithYML(t *testing.T) {
	sdkRouter := New(config.GetLbrynetServers())
	assert.True(t, len(sdkRouter.GetAll()) > 0, "No servers")
}

func TestFirstServer(t *testing.T) {
	t.Skip("maps are not ordered")
	config.Override("LbrynetServers", map[string]string{
		"default": "http://lbrynet1:5279/",
		"sdk1":    "http://lbrynet2:5279/",
		"sdk2":    "http://lbrynet3:5279/",
	})
	defer config.RestoreOverridden()
	sdkRouter := New(config.GetLbrynetServers())
	server := sdkRouter.GetServer("lbrytv-id.756130.wallet").Address
	assert.Equal(t, "http://lbrynet1:5279/", server)

	server = sdkRouter.GetServer("lbrytv-id.1767731.wallet").Address
	assert.Equal(t, "http://lbrynet2:5279/", server)

	server = sdkRouter.GetServer("lbrytv-id.751365.wallet").Address
	assert.Equal(t, "http://lbrynet3:5279/", server)
}

func TestServerRetrieval(t *testing.T) {
	config.Override("LbrynetServers", map[string]string{
		"default": "http://lbrynet1:5279/",
		"sdk1":    "http://lbrynet2:5279/",
		"sdk2":    "http://lbrynet3:5279/",
	})
	defer config.RestoreOverridden()
	sdkRouter := New(config.GetLbrynetServers())
	servers := sdkRouter.GetAll()
	for i := 0; i < 10000; i++ {
		walletID := wallet.MakeID(i)
		server := sdkRouter.GetServer(walletID).Address
		assert.Equal(t, servers[i%10%3].Address, server)
	}
}

func TestDefaultLbrynetServer(t *testing.T) {
	sdkRouter := New(config.GetLbrynetServers())
	for _, s := range sdkRouter.servers {
		if s.Name == "default" { // QUESTION: should I use router.DefaultServer constant in tests? or should this stay a string?
			return
		}
	}
	t.Error("No default lbrynet server is specified in the lbrytv.yml")
}

func TestMain(m *testing.M) {
	dbConfig := config.GetDatabase()
	params := storage.ConnParams{
		Connection: dbConfig.Connection,
		DBName:     dbConfig.DBName,
		Options:    dbConfig.Options,
	}
	dbConn, connCleanup := storage.CreateTestConn(params)
	dbConn.SetDefaultConnection()
	defer connCleanup()

	code := m.Run()

	os.Exit(code)
}

func TestOverrideLbrynetDefault(t *testing.T) {
	config.Override("LbrynetServers", map[string]string{"default": "http://localhost:5279"})
	defer config.RestoreOverridden()
	sdkRouter := New(config.GetLbrynetServers())
	dummyID := 2343465345
	wallet.MakeID(dummyID)
	server := sdkRouter.GetServer(wallet.MakeID(dummyID))
	assert.Equal(t, server.Address, "http://localhost:5279")
}

func TestOverrideLbrynet(t *testing.T) {
	config.Override("Lbrynet", "http://localhost:5279")
	config.Override("LbrynetServers", map[string]string{})
	defer config.RestoreOverridden()
	sdkRouter := New(config.GetLbrynetServers())
	dummyID := 2343465345
	wallet.MakeID(dummyID)
	server := sdkRouter.GetServer(wallet.MakeID(dummyID))
	assert.Equal(t, server.Address, "http://localhost:5279")
}

func TestGetUserID(t *testing.T) {
	walletID := "sjdfkjhsdkjs.1234235.sdfsgf"
	userID := getUserID(walletID)
	assert.Equal(t, 1234235, userID)
}
