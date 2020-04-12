package lbrynet

import (
	"errors"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/config"
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

	wid, err := InitializeWallet(r, uid)
	require.NoError(t, err)
	assert.Equal(t, wid, sdkrouter.WalletID(uid))

	err = UnloadWallet(r, uid)
	require.NoError(t, err)

	wid, err = InitializeWallet(r, uid)
	require.NoError(t, err)
	assert.Equal(t, wid, sdkrouter.WalletID(uid))
}

func TestCreateWalletLoadWallet(t *testing.T) {
	uid := rand.Int()
	r := sdkrouter.New(config.GetLbrynetServers())

	w, err := createWallet(r, uid)
	require.NoError(t, err)
	assert.Equal(t, w.ID, sdkrouter.WalletID(uid))

	_, err = createWallet(r, uid)
	require.NotNil(t, err)
	assert.True(t, errors.Is(err, ErrWalletExists))

	err = UnloadWallet(r, uid)
	require.NoError(t, err)

	w, err = loadWallet(r, uid)
	require.NoError(t, err)
	assert.Equal(t, w.ID, sdkrouter.WalletID(uid))
}
