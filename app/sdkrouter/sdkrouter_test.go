package sdkrouter

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/lbrynet"
	"github.com/lbryio/lbrytv/internal/storage"
	"github.com/lbryio/lbrytv/internal/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	servers := map[string]string{
		// internally, servers will be sorted in lexical order by name
		"b": "1",
		"a": "0",
		"d": "3",
		"c": "2",
	}
	r := New(servers)

	for i := 1; i < 100; i++ {
		server := r.GetServer(i).Address
		assert.Equal(t, fmt.Sprintf("%d", i%len(servers)), server)
	}
}

func TestOverrideLbrynetDefaultConf(t *testing.T) {
	address := "http://space.com:1234"
	config.Override("LbrynetServers", map[string]string{"x": address})
	defer config.RestoreOverridden()
	server := New(config.GetLbrynetServers()).GetServer(343465345)
	assert.Equal(t, address, server.Address)
}

func TestOverrideLbrynetConf(t *testing.T) {
	address := "http://turtles.network"
	config.Override("Lbrynet", address)
	config.Override("LbrynetServers", map[string]string{})
	defer config.RestoreOverridden()
	server := New(config.GetLbrynetServers()).GetServer(1343465345)
	assert.Equal(t, address, server.Address)
}

func TestGetUserID(t *testing.T) {
	assert.Equal(t, 1234235, UserID("sjdfkjhsdkjs.1234235.sdfsgf"))
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

func TestInitializeWallet(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	userID := rand.Int()
	r := New(config.GetLbrynetServers())

	walletID, err := r.InitializeWallet(userID)
	require.NoError(t, err)
	assert.Equal(t, walletID, WalletID(userID))

	err = r.UnloadWallet(userID)
	require.NoError(t, err)

	walletID, err = r.InitializeWallet(userID)
	require.NoError(t, err)
	assert.Equal(t, walletID, WalletID(userID))
}

func TestCreateWalletLoadWallet(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	userID := rand.Int()
	r := New(config.GetLbrynetServers())

	wallet, err := r.createWallet(userID)
	require.NoError(t, err)
	assert.Equal(t, wallet.ID, WalletID(userID))

	wallet, err = r.createWallet(userID)
	require.NotNil(t, err)
	assert.True(t, errors.Is(err, lbrynet.ErrWalletExists))

	err = r.UnloadWallet(userID)
	require.NoError(t, err)

	wallet, err = r.loadWallet(userID)
	require.NoError(t, err)
	assert.Equal(t, wallet.ID, WalletID(userID))
}
