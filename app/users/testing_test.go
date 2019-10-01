package users

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTestUserRetrieverGetWalletID(t *testing.T) {
	var (
		testAuth *Authenticator
		r        *http.Request
		err      error
		a        string
	)

	testAuth = NewAuthenticator(&TestUserRetriever{WalletID: "123"})
	r, _ = http.NewRequest("GET", "/", nil)
	r.Header.Set(TokenHeader, "XyZ")
	a, err = testAuth.GetWalletID(r)
	assert.Nil(t, err)
	assert.Equal(t, "123", a)

	r, _ = http.NewRequest("GET", "/", nil)
	r.Header.Set(TokenHeader, "aBc")
	a, err = testAuth.GetWalletID(r)
	assert.Nil(t, err)
	assert.Equal(t, "123", a)

	testAuth = NewAuthenticator(&TestUserRetriever{WalletID: "123", Token: "XyZ"})
	r, _ = http.NewRequest("GET", "/", nil)
	r.Header.Set(TokenHeader, "XyZ")
	a, err = testAuth.GetWalletID(r)
	assert.Nil(t, err)
	assert.Equal(t, "123", a)

	r, _ = http.NewRequest("GET", "/", nil)
	r.Header.Set(TokenHeader, "aBc")
	a, err = testAuth.GetWalletID(r)
	assert.Equal(t, errors.New(GenericRetrievalErr), err)
	assert.Equal(t, "", a)

	r, _ = http.NewRequest("GET", "/", nil)
	a, err = testAuth.GetWalletID(r)
	assert.Nil(t, err)
	assert.Equal(t, "", a)

}
