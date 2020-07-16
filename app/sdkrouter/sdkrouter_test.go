package sdkrouter

import (
	"os"
	"testing"

	"github.com/lbryio/lbrytv/apps/lbrytv/config"
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

	code := m.Run()

	connCleanup()
	os.Exit(code)
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
	rpcServer1 := test.MockHTTPServer(nil)
	defer rpcServer1.Close()
	rpcServer2 := test.MockHTTPServer(nil)
	defer rpcServer2.Close()
	rpcServer3 := test.MockHTTPServer(nil)
	defer rpcServer3.Close()

	servers := map[string]string{
		"srv1": rpcServer1.URL,
		"srv2": rpcServer2.URL,
		"srv3": rpcServer3.URL,
	}
	r := New(servers)

	// try doing the load in increasing order
	rpcServer1.NextResponse <- `{"result":{"total_pages":1}}`
	rpcServer2.NextResponse <- `{"result":{"total_pages":2}}`
	rpcServer3.NextResponse <- `{"result":{"total_pages":3}}`
	r.updateLoadAndMetrics()
	assert.Equal(t, "srv1", r.LeastLoaded().Name)

	// now do the load in decreasing order
	rpcServer1.NextResponse <- `{"result":{"total_pages":3}}`
	rpcServer2.NextResponse <- `{"result":{"total_pages":2}}`
	rpcServer3.NextResponse <- `{"result":{"total_pages":1}}`
	r.updateLoadAndMetrics()
	assert.Equal(t, "srv3", r.LeastLoaded().Name)

}
