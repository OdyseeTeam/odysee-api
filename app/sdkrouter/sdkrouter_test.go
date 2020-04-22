package sdkrouter

import (
	"fmt"
	"os"
	"testing"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/storage"
	"github.com/lbryio/lbrytv/internal/test"

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
	r := New(config.GetLbrynetServers())
	assert.True(t, len(r.GetAll()) > 0, "No servers")
}

func TestServerOrder(t *testing.T) {
	t.Skip("might bring this back when servers have an order")
}

func TestOverrideLbrynetDefaultConf(t *testing.T) {
	address := "http://space.com:1234"
	config.Override("LbrynetServers", map[string]string{"x": address})
	defer config.RestoreOverridden()
	server := New(config.GetLbrynetServers()).RandomServer()
	assert.Equal(t, address, server.Address)
}

func TestOverrideLbrynetConf(t *testing.T) {
	address := "http://turtles.network"
	config.Override("Lbrynet", address)
	config.Override("LbrynetServers", map[string]string{})
	defer config.RestoreOverridden()
	server := New(config.GetLbrynetServers()).RandomServer()
	assert.Equal(t, address, server.Address)
}

func TestLeastLoaded(t *testing.T) {
	rpcServer := test.MockHTTPServer(nil)
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
			rpcServer.NextResponse <- fmt.Sprintf(`{"result":{"total_pages":%d}}`, i)
		}
	}()
	r.updateLoadAndMetrics()
	assert.Equal(t, "srv1", r.LeastLoaded().Name)

	// now do the load in decreasing order
	go func() {
		for i := 0; i < len(servers); i++ {
			rpcServer.NextResponse <- fmt.Sprintf(`{"result":{"total_pages":%d}}`, len(servers)-i)
		}
	}()
	r.updateLoadAndMetrics()
	assert.Equal(t, "srv3", r.LeastLoaded().Name)

}
