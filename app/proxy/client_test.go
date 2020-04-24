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

func TestClient_CallQueryWithRetry(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	dummyUserID := rand.Intn(100)
	addr := test.RandServerAddress(t)

	err := wallet.Create(addr, dummyUserID)
	require.NoError(t, err)
	err = wallet.UnloadWallet(addr, dummyUserID)
	require.NoError(t, err)

	q, err := NewQuery(jsonrpc.NewRequest("wallet_balance"))
	require.NoError(t, err)
	q.WalletID = sdkrouter.WalletID(dummyUserID)

	// check that sdk loads the wallet and retries the query if the wallet was not initially loaded

	c := NewCaller(addr, dummyUserID)
	r, err := c.callQueryWithRetry(q)
	require.NoError(t, err)
	require.Nil(t, r.Error)
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

	srv.QueueResponses(
		test.ResToStr(t, jsonrpc.RPCResponse{
			JSONRPC: "2.0",
			Error: &jsonrpc.RPCError{
				Message: "Couldn't find wallet: //",
			},
		}),
		test.EmptyResponse(), // for the wallet_add call
		test.ResToStr(t, jsonrpc.RPCResponse{
			JSONRPC: "2.0",
			Error: &jsonrpc.RPCError{
				Message: "Wallet at path // was not found",
			},
		}),
	)

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

	srv.QueueResponses(
		test.ResToStr(t, jsonrpc.RPCResponse{
			JSONRPC: "2.0",
			Error: &jsonrpc.RPCError{
				Message: "Couldn't find wallet: //",
			},
		}),
		test.EmptyResponse(), // for the wallet_add call
		test.ResToStr(t, jsonrpc.RPCResponse{
			JSONRPC: "2.0",
			Error: &jsonrpc.RPCError{
				Message: "Wallet at path // is already loaded",
			},
		}),
		test.ResToStr(t, jsonrpc.RPCResponse{
			JSONRPC: "2.0",
			Result:  `"99999.00"`,
		}),
	)

	r, err := c.callQueryWithRetry(q)

	require.NoError(t, err)
	require.Nil(t, r.Error)
	require.Equal(t, `"99999.00"`, r.Result)
}
