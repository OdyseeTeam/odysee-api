package users

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lbryio/lbrytv/models"

	"github.com/stretchr/testify/assert"
)

type DummyRetriever struct {
	remoteIP string
}

func (r *DummyRetriever) Retrieve(q Query) (*models.User, error) {
	r.remoteIP = q.MetaRemoteIP
	if q.Token == "XyZ" {
		return &models.User{WalletID: "aBc"}, nil
	}
	return nil, errors.New("cannot authenticate")
}

type UnverifiedRetriever struct {
	remoteIP string
}

func (r *UnverifiedRetriever) Retrieve(q Query) (*models.User, error) {
	return nil, nil
}

func AuthenticatedHandler(w http.ResponseWriter, r *AuthenticatedRequest) {
	if r.IsAuthenticated() {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(r.WalletID))
	} else {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(r.AuthError.Error()))
	}
}

func TestAuthenticator(t *testing.T) {
	retriever := &DummyRetriever{}
	r, _ := http.NewRequest("GET", "/api/proxy", nil)
	r.Header.Set(TokenHeader, "XyZ")
	r.Header.Set("X-Forwarded-For", "8.8.8.8")

	rr := httptest.NewRecorder()
	authenticator := NewAuthenticator(retriever)

	http.HandlerFunc(authenticator.Wrap(AuthenticatedHandler)).ServeHTTP(rr, r)

	response := rr.Result()
	body, _ := ioutil.ReadAll(response.Body)
	assert.Equal(t, "aBc", string(body))
	assert.Equal(t, "8.8.8.8", retriever.remoteIP)
}

func TestAuthenticatorFailure(t *testing.T) {
	r, _ := http.NewRequest("GET", "/api/proxy", nil)
	r.Header.Set(TokenHeader, "ALSDJ")
	rr := httptest.NewRecorder()

	authenticator := NewAuthenticator(&DummyRetriever{})

	http.HandlerFunc(authenticator.Wrap(AuthenticatedHandler)).ServeHTTP(rr, r)
	response := rr.Result()
	body, _ := ioutil.ReadAll(response.Body)
	assert.Equal(t, "cannot authenticate", string(body))
	assert.Equal(t, http.StatusForbidden, response.StatusCode)
}

func TestAuthenticatorGetWalletIDUnverifiedUser(t *testing.T) {
	r, _ := http.NewRequest("GET", "/api/proxy", nil)
	r.Header.Set(TokenHeader, "zzz")

	a := NewAuthenticator(&UnverifiedRetriever{})

	wid, err := a.GetWalletID(r)
	assert.NoError(t, err)
	assert.Equal(t, "", wid)
}
