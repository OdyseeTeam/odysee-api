package users

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTestUserRetrieverGetAccountID(t *testing.T) {
	var (
		testAuth *Authenticator
		r        *http.Request
		err      error
		a        string
	)

	testAuth = NewAuthenticator(&TestUserRetriever{AccountID: "123"})
	r, _ = http.NewRequest("GET", "/", nil)
	r.Header.Set(TokenHeader, "XyZ")
	a, err = testAuth.GetAccountID(r)
	assert.Nil(t, err)
	assert.Equal(t, "123", a)

	r, _ = http.NewRequest("GET", "/", nil)
	r.Header.Set(TokenHeader, "aBc")
	a, err = testAuth.GetAccountID(r)
	assert.Nil(t, err)
	assert.Equal(t, "123", a)

	testAuth = NewAuthenticator(&TestUserRetriever{AccountID: "123", Token: "XyZ"})
	r, _ = http.NewRequest("GET", "/", nil)
	r.Header.Set(TokenHeader, "XyZ")
	a, err = testAuth.GetAccountID(r)
	assert.Nil(t, err)
	assert.Equal(t, "123", a)

	r, _ = http.NewRequest("GET", "/", nil)
	r.Header.Set(TokenHeader, "aBc")
	a, err = testAuth.GetAccountID(r)
	assert.Equal(t, errors.New(GenericRetrievalErr), err)
	assert.Equal(t, "", a)

	r, _ = http.NewRequest("GET", "/", nil)
	a, err = testAuth.GetAccountID(r)
	assert.Nil(t, err)
	assert.Equal(t, "", a)

}
