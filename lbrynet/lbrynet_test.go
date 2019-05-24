package lbrynet

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func prettyPrint(i interface{}) {
	s, _ := json.MarshalIndent(i, "", "\t")
	fmt.Println(string(s))
}

func generateTestEmail() string {
	return "test" + fmt.Sprintf("%d%d", time.Now().Unix(), rand.Int()) + "@lbry.com"
}

func TestGetAccount(t *testing.T) {
	email := generateTestEmail()
	account, err := CreateAccount(email)
	require.Nil(t, err, err)

	retrievedAccount, err := GetAccount(email)
	require.Nil(t, err, err)
	assert.Equal(t, retrievedAccount.Name, account.Name)
	assert.Equal(t, retrievedAccount.ID, account.ID)
	prettyPrint(retrievedAccount)
	prettyPrint(account)
}

func TestGetAccount_Nonexistent(t *testing.T) {
	email := generateTestEmail()
	retrievedAccount, err := GetAccount(email)
	assert.IsType(t, AccountNotFound{}, err)
	assert.Nil(t, retrievedAccount)
}

func TestCreateAccount(t *testing.T) {
	email := generateTestEmail()

	account, err := CreateAccount(email)
	prettyPrint(account)

	require.Nil(t, err, err)
	assert.Equal(t, accountNamePrefix+email, account.Name)
}

func TestCreateAccount_DuplicateNotAllowed(t *testing.T) {
	email := generateTestEmail()

	account, err := CreateAccount(email)
	require.Nil(t, err)

	account, err = CreateAccount(email)
	assert.NotNil(t, err)
	assert.Nil(t, account)
}

func BenchmarkCreateAccount(b *testing.B) {
	emails := [100]string{}
	for x := range [100]int{} {
		emails[x] = generateTestEmail()
		_, err := CreateAccount(emails[x])
		require.Nil(b, err)
	}
}
