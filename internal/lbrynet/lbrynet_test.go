package lbrynet

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/util/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func prettyPrint(i interface{}) {
	s, _ := json.MarshalIndent(i, "", "\t")
	fmt.Println(string(s))
}

func generateTestUID() int {
	return rand.Int()
}

func TestMain(m *testing.M) {
	rand.Seed(time.Now().UnixNano())
	code := m.Run()
	os.Exit(code)
}

func TestResolve(t *testing.T) {
	r, err := Resolve("what#6769855a9aa43b67086f9ff3c1a5bacb5698a27a")
	prettyPrint(r)

	require.Nil(t, err)
	require.NotNil(t, r)
}

func TestInitializeWallet(t *testing.T) {
	uid := rand.Int()

	_, wid, err := InitializeWallet(uid)
	require.Nil(t, err)
	assert.Equal(t, wid, wallet.MakeID(uid))

	_, err = WalletRemove(uid)
	require.Nil(t, err)

	_, wid, err = InitializeWallet(uid)
	require.Nil(t, err)
	assert.Equal(t, wid, wallet.MakeID(uid))
}

func TestCreateWalletAddWallet(t *testing.T) {
	uid := rand.Int()

	w, _, err := CreateWallet(uid)
	require.Nil(t, err)
	assert.Equal(t, w.ID, wallet.MakeID(uid))

	_, _, err = CreateWallet(uid)
	require.NotNil(t, err)
	assert.True(t, errors.As(err, &WalletExists{}))

	_, err = WalletRemove(uid)
	require.Nil(t, err)

	w, err = AddWallet(uid)
	require.Nil(t, err)
	assert.Equal(t, w.ID, wallet.MakeID(uid))
}
