package lbrynet

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

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

func TestGetAccount(t *testing.T) {
	uid := generateTestUID()
	account, err := CreateAccount(uid)
	require.Nil(t, err, err)

	retrievedAccount, err := GetAccount(uid)
	require.Nil(t, err, err)
	assert.Equal(t, retrievedAccount.Name, account.Name)
	assert.Equal(t, retrievedAccount.ID, account.ID)
	prettyPrint(retrievedAccount)
	prettyPrint(account)
}

func TestGetAccount_Nonexistent(t *testing.T) {
	uid := generateTestUID()
	retrievedAccount, err := GetAccount(uid)
	assert.IsType(t, AccountNotFound{}, err)
	assert.Nil(t, retrievedAccount)
}

func TestCreateAccount(t *testing.T) {
	uid := generateTestUID()

	account, err := CreateAccount(uid)
	prettyPrint(account)

	require.Nil(t, err, err)
	assert.Equal(t, fmt.Sprintf("%v%v", accountNamePrefix, uid), account.Name)
}

func TestCreateAccount_DuplicateNotAllowed(t *testing.T) {
	uid := generateTestUID()

	account, err := CreateAccount(uid)
	require.Nil(t, err)

	account, err = CreateAccount(uid)
	assert.NotNil(t, err)
	assert.Nil(t, account)
}

func TestResolve(t *testing.T) {
	r, err := Resolve("what#6769855a9aa43b67086f9ff3c1a5bacb5698a27a")
	prettyPrint(r)

	require.Nil(t, err)
	require.NotNil(t, r)
}

func TestInitializeWallet(t *testing.T) {
	uid := rand.Int()

	wid, err := InitializeWallet(uid)
	require.Nil(t, err)
	assert.Equal(t, wid, MakeWalletID(uid))

	_, err = WalletRemove(uid)
	require.Nil(t, err)

	wid, err = InitializeWallet(uid)
	require.Nil(t, err)
	assert.Equal(t, wid, MakeWalletID(uid))
}

func TestCreateWalletAddWallet(t *testing.T) {
	uid := rand.Int()

	w, err := CreateWallet(uid)
	require.Nil(t, err)
	assert.Equal(t, w.ID, MakeWalletID(uid))

	_, err = CreateWallet(uid)
	require.NotNil(t, err)
	assert.True(t, errors.As(err, &WalletExists{}))

	_, err = WalletRemove(uid)
	require.Nil(t, err)

	w, err = AddWallet(uid)
	require.Nil(t, err)
	assert.Equal(t, w.ID, MakeWalletID(uid))
}

func BenchmarkCreateAccount(b *testing.B) {
	uids := [100]int{}
	for i := 0; i <= len(uids); i++ {
		uids[i] = generateTestUID()
		_, err := CreateAccount(uids[i])
		require.Nil(b, err)
	}
	for i := 0; i <= len(uids); i++ {
		_, err := RemoveAccount(uids[i])
		require.Nil(b, err)
	}
}
