package users

import (
	"errors"
	"net/http"
	"testing"

	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestUserRetrieverGetWalletID(t *testing.T) {
	testAuth := &Authenticator{Retriever: DummyRetriever("", "123")}
	r, err := http.NewRequest("GET", "/", nil)
	require.NoError(t, err)
	r.Header.Set(wallet.TokenHeader, "XyZ")
	a, err := testAuth.GetWalletID(r)
	assert.NoError(t, err)
	assert.Equal(t, "123", a)

	r, _ = http.NewRequest("GET", "/", nil)
	r.Header.Set(wallet.TokenHeader, "aBc")
	a, err = testAuth.GetWalletID(r)
	assert.NoError(t, err)
	assert.Equal(t, "123", a)

	testAuth = &Authenticator{Retriever: DummyRetriever("XyZ", "123")}
	r, _ = http.NewRequest("GET", "/", nil)
	r.Header.Set(wallet.TokenHeader, "XyZ")
	a, err = testAuth.GetWalletID(r)
	assert.NoError(t, err)
	assert.Equal(t, "123", a)

	r, _ = http.NewRequest("GET", "/", nil)
	r.Header.Set(wallet.TokenHeader, "aBc")
	a, err = testAuth.GetWalletID(r)
	assert.Equal(t, errors.New(GenericRetrievalErr), err)
	assert.Equal(t, "", a)

	r, _ = http.NewRequest("GET", "/", nil)
	a, err = testAuth.GetWalletID(r)
	assert.NoError(t, err)
	assert.Equal(t, "", a)

}
