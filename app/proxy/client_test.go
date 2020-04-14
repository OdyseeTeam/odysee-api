package proxy

import (
	"math/rand"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/internal/test"

	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

func TestClientCallDoesReloadWallet(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	dummyUserID := rand.Intn(100)
	addr := test.RandServerAddress(t)

	walletID, err := wallet.Create(addr, dummyUserID)
	require.NoError(t, err)
	err = wallet.UnloadWallet(addr, dummyUserID)
	require.NoError(t, err)

	q, err := NewQuery(jsonrpc.NewRequest("wallet_balance"))
	require.NoError(t, err)
	q.WalletID = walletID

	c := NewCaller(addr, dummyUserID)
	r, err := c.callQueryWithRetry(q)
	// err = json.Unmarshal(result, response)
	require.NoError(t, err)
	require.Nil(t, r.Error)

	// TODO: check that wallet is actually reloaded? what is this test even testing?
}

func TestClientCallDoesNotReloadWalletAfterOtherErrors(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	walletID := sdkrouter.WalletID(rand.Intn(100))

	srv := test.MockHTTPServer(nil)
	defer srv.Close()

	c := NewCaller(srv.URL, 0)
	q, err := NewQuery(jsonrpc.NewRequest("wallet_balance"))
	require.NoError(t, err)
	q.WalletID = walletID

	go func() {
		srv.NextResponse <- test.ResToStr(t, jsonrpc.RPCResponse{
			JSONRPC: "2.0",
			Error: &jsonrpc.RPCError{
				Message: "Couldn't find wallet: //",
			},
		})
		srv.RespondWithNothing() // for the wallet_add call
		srv.NextResponse <- test.ResToStr(t, jsonrpc.RPCResponse{
			JSONRPC: "2.0",
			Error: &jsonrpc.RPCError{
				Message: "Wallet at path // was not found",
			},
		})
	}()

	r, err := c.callQueryWithRetry(q)
	require.NoError(t, err)
	require.Equal(t, "Wallet at path // was not found", r.Error.Message)
}

func TestClientCallDoesNotReloadWalletIfAlreadyLoaded(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	walletID := sdkrouter.WalletID(rand.Intn(100))

	srv := test.MockHTTPServer(nil)
	defer srv.Close()

	c := NewCaller(srv.URL, 0)
	q, err := NewQuery(jsonrpc.NewRequest("wallet_balance"))
	require.NoError(t, err)
	q.WalletID = walletID

	go func() {
		srv.NextResponse <- test.ResToStr(t, jsonrpc.RPCResponse{
			JSONRPC: "2.0",
			Error: &jsonrpc.RPCError{
				Message: "Couldn't find wallet: //",
			},
		})
		srv.RespondWithNothing() // for the wallet_add call
		srv.NextResponse <- test.ResToStr(t, jsonrpc.RPCResponse{
			JSONRPC: "2.0",
			Error: &jsonrpc.RPCError{
				Message: "Wallet at path // is already loaded",
			},
		})
		srv.NextResponse <- test.ResToStr(t, jsonrpc.RPCResponse{
			JSONRPC: "2.0",
			Result:  `"99999.00"`,
		})
	}()

	r, err := c.callQueryWithRetry(q)

	require.NoError(t, err)
	require.Nil(t, r.Error)
	require.Equal(t, `"99999.00"`, r.Result)
}
