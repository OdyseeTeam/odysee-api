package sdkrouter

import (
	"os"
	"testing"

	"github.com/lbryio/lbrytv/apps/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/storage"
	"github.com/lbryio/lbrytv/internal/test"
	"github.com/lbryio/lbrytv/models"

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
	rpcServerPvt := test.MockHTTPServer(nil)
	defer rpcServerPvt.Close()

	servers := []*models.LbrynetServer{
		{Name: "srv1", Address: rpcServer1.URL},
		{Name: "srv2", Address: rpcServer2.URL},
		{Name: "srv3", Address: rpcServer3.URL},
		{Name: "srv-pvt", Address: rpcServerPvt.URL, Private: true},
	}
	r := NewWithServers(servers...)

	// try doing the load in increasing order
	rpcServerPvt.NextResponse <- `{"result":{"total_pages":5}}`
	rpcServer1.NextResponse <- `{"result":{"total_pages":10}}`
	rpcServer2.NextResponse <- `{"result":{"total_pages":20}}`
	rpcServer3.NextResponse <- `{"result":{"total_pages":30}}`
	r.updateLoadAndMetrics()
	assert.Equal(t, "srv1", r.LeastLoaded().Name)

	// now do the load in decreasing order
	rpcServer1.NextResponse <- `{"result":{"total_pages":30}}`
	rpcServer2.NextResponse <- `{"result":{"total_pages":20}}`
	rpcServer3.NextResponse <- `{"result":{"total_pages":10}}`
	rpcServerPvt.NextResponse <- `{"result":{"total_pages":5}}`
	r.updateLoadAndMetrics()
	assert.Equal(t, "srv3", r.LeastLoaded().Name)

}
