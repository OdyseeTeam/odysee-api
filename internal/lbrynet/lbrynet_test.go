package lbrynet

import (
	"errors"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/util/wallet"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	rand.Seed(time.Now().UnixNano())
	code := m.Run()
	os.Exit(code)
}

func TestInitializeWallet(t *testing.T) {
	uid := rand.Int()
	r := sdkrouter.New(config.GetLbrynetServers())

	_, wid, err := InitializeWallet(r, uid)
	require.NoError(t, err)
	assert.Equal(t, wid, wallet.MakeID(uid))

	_, err = WalletRemove(r, uid)
	require.NoError(t, err)

	_, wid, err = InitializeWallet(r, uid)
	require.NoError(t, err)
	assert.Equal(t, wid, wallet.MakeID(uid))
}

func TestCreateWalletAddWallet(t *testing.T) {
	uid := rand.Int()
	r := sdkrouter.New(config.GetLbrynetServers())

	w, _, err := CreateWallet(r, uid)
	require.NoError(t, err)
	assert.Equal(t, w.ID, wallet.MakeID(uid))

	_, _, err = CreateWallet(r, uid)
	require.NotNil(t, err)
	assert.True(t, errors.As(err, &WalletExists{}))

	_, err = WalletRemove(r, uid)
	require.NoError(t, err)

	w, err = AddWallet(r, uid)
	require.NoError(t, err)
	assert.Equal(t, w.ID, wallet.MakeID(uid))
}
