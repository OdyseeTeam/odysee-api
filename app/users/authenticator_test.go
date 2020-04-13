package users

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func authedHandler(w http.ResponseWriter, r *AuthenticatedRequest) {
	if r.IsAuthenticated() {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(r.WalletID))
	} else {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(r.AuthError.Error()))
	}
}

func TestAuthenticator(t *testing.T) {
	r, err := http.NewRequest("GET", "/api/proxy", nil)
	require.NoError(t, err)
	r.Header.Set(wallet.TokenHeader, "XyZ")
	r.Header.Set("X-Forwarded-For", "8.8.8.8")

	var receivedRemoteIP string
	authenticator := &Authenticator{
		Retriever: func(token, ip string) (*models.User, error) {
			receivedRemoteIP = ip
			if token == "XyZ" {
				return &models.User{WalletID: "aBc"}, nil
			}
			return nil, errors.New(GenericRetrievalErr)
		},
	}

	rr := httptest.NewRecorder()
	authenticator.Wrap(authedHandler).ServeHTTP(rr, r)
	response := rr.Result()
	body, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, "aBc", string(body))
	assert.Equal(t, "8.8.8.8", receivedRemoteIP)
}

func TestAuthenticatorFailure(t *testing.T) {
	r, err := http.NewRequest("GET", "/api/proxy", nil)
	require.NoError(t, err)
	r.Header.Set(wallet.TokenHeader, "ALSDJ")
	rr := httptest.NewRecorder()

	authenticator := &Authenticator{Retriever: DummyRetriever("XyZ", "")}

	authenticator.Wrap(authedHandler).ServeHTTP(rr, r)
	response := rr.Result()
	body, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, GenericRetrievalErr, string(body))
	assert.Equal(t, http.StatusForbidden, response.StatusCode)
}

func TestAuthenticatorGetWalletIDUnverifiedUser(t *testing.T) {
	r, err := http.NewRequest("GET", "/api/proxy", nil)
	require.NoError(t, err)
	r.Header.Set(wallet.TokenHeader, "zzz")

	a := &Authenticator{Retriever: func(token, ip string) (*models.User, error) { return nil, nil }}

	walletID, err := a.GetWalletID(r)
	assert.NoError(t, err)
	assert.Equal(t, "", walletID)
}
