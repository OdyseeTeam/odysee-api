package lbrynet

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/internal/test"
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

func TestInitializeWallet(t *testing.T) {
	uid := rand.Int()
	r := test.SDKRouter()

	_, wid, err := InitializeWallet(r, uid)
	require.Nil(t, err)
	assert.Equal(t, wid, wallet.MakeID(uid))

	_, err = WalletRemove(r, uid)
	require.Nil(t, err)

	_, wid, err = InitializeWallet(r, uid)
	require.Nil(t, err)
	assert.Equal(t, wid, wallet.MakeID(uid))
}

func TestCreateWalletAddWallet(t *testing.T) {
	uid := rand.Int()
	r := test.SDKRouter()

	w, _, err := CreateWallet(r, uid)
	require.Nil(t, err)
	assert.Equal(t, w.ID, wallet.MakeID(uid))

	_, _, err = CreateWallet(r, uid)
	require.NotNil(t, err)
	assert.True(t, errors.As(err, &WalletExists{}))

	_, err = WalletRemove(r, uid)
	require.Nil(t, err)

	w, err = AddWallet(r, uid)
	require.Nil(t, err)
	assert.Equal(t, w.ID, wallet.MakeID(uid))
}
