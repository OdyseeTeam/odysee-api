package sdkrouter

import (
	"fmt"
	"os"
	"testing"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/storage"
	"github.com/lbryio/lbrytv/internal/test"
	"github.com/lbryio/lbrytv/util/wallet"

	"github.com/stretchr/testify/assert"
)

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
	os.Exit(m.Run())
}

func TestInitializeWithYML(t *testing.T) {
	sdkRouter := New(config.GetLbrynetServers())
	assert.True(t, len(sdkRouter.GetAll()) > 0, "No servers")
}

func TestServerOrder(t *testing.T) {
	servers := map[string]string{
		// internally, servers will be sorted in lexical order by name
		"b": "1",
		"a": "0",
		"d": "3",
		"c": "2",
	}
	sdkRouter := New(servers)

	for i := 0; i < 100; i++ {
		server := sdkRouter.GetServer(wallet.MakeID(i)).Address
		assert.Equal(t, fmt.Sprintf("%d", i%len(servers)), server)
	}
}

func TestOverrideLbrynetDefaultConf(t *testing.T) {
	address := "http://space.com:1234"
	config.Override("LbrynetServers", map[string]string{"x": address})
	defer config.RestoreOverridden()
	server := New(config.GetLbrynetServers()).GetServer(wallet.MakeID(343465345))
	assert.Equal(t, address, server.Address)
}

func TestOverrideLbrynetConf(t *testing.T) {
	address := "http://turtles.network"
	config.Override("Lbrynet", address)
	config.Override("LbrynetServers", map[string]string{})
	defer config.RestoreOverridden()
	server := New(config.GetLbrynetServers()).GetServer(wallet.MakeID(1343465345))
	assert.Equal(t, address, server.Address)
}

func TestGetUserID(t *testing.T) {
	userID := getUserID("sjdfkjhsdkjs.1234235.sdfsgf")
	assert.Equal(t, 1234235, userID)
}

func TestLeastLoaded(t *testing.T) {
	reqChan := make(chan *test.RequestData, 1)
	rpcServer, nextResp := test.MockJSONRPCServer(reqChan)
	defer rpcServer.Close()

	servers := map[string]string{
		"srv1": rpcServer.URL,
		"srv2": rpcServer.URL,
		"srv3": rpcServer.URL,
	}
	r := New(servers)

	// try doing the load in increasing order
	go func() {
		for i := 0; i < len(servers); i++ {
			nextResp(fmt.Sprintf(`{"result":{"total_pages":%d}}`, i))
			<-reqChan
		}
	}()
	r.updateLoadAndMetrics()
	assert.Equal(t, "srv1", r.LeastLoaded().Name)

	// now do the load in decreasing order
	go func() {
		for i := 0; i < len(servers); i++ {
			nextResp(fmt.Sprintf(`{"result":{"total_pages":%d}}`, len(servers)-i))
			<-reqChan
		}
	}()
	r.updateLoadAndMetrics()
	assert.Equal(t, "srv3", r.LeastLoaded().Name)

}
