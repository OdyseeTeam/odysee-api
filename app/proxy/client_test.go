package proxy

import (
	"math/rand"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/app/router"

	"github.com/lbryio/lbrytv/internal/lbrynet"

	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

func TestClientCallDoesReloadWallet(t *testing.T) {
	var (
		r *jsonrpc.RPCResponse
	)

	rand.Seed(time.Now().UnixNano())
	dummyUserID := rand.Intn(100)

	_, wid, _ := lbrynet.InitializeWallet(dummyUserID)
	_, err := lbrynet.WalletRemove(dummyUserID)
	require.NoError(t, err)

	router := router.NewDefault()

	c := NewClient(router.GetSDKServerAddress(wid), wid, time.Second*1)

	q, _ := NewQuery(newRawRequest(t, "wallet_balance", nil))
	q.SetWalletID(wid)
	r, err = c.Call(q)

	// err = json.Unmarshal(result, response)
	require.NoError(t, err)
	require.Nil(t, r.Error)
}
