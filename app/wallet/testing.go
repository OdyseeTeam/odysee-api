package wallet

import (
	"net/http"

	"github.com/OdyseeTeam/odysee-api/models"

	"github.com/volatiletech/null"
)

type Authenticator interface {
	Authenticate(token, metaRemoteIP string) (*models.User, error)
	GetTokenFromRequest(r *http.Request) (string, error)
}

// TestAnyAuthenticator will authenticate any token and return a dummy user.
type TestAnyAuthenticator struct{}

func (a *TestAnyAuthenticator) Authenticate(token, ip string) (*models.User, error) {
	return &models.User{ID: 994, IdpID: null.StringFrom("my-idp-id")}, nil
}

func (a *TestAnyAuthenticator) GetTokenFromRequest(r *http.Request) (string, error) {
	return "", nil
}

// TestMissingTokenAuthenticator will throw a missing token error.
type TestMissingTokenAuthenticator struct{}

func (a *TestMissingTokenAuthenticator) Authenticate(token, ip string) (*models.User, error) {
	return nil, nil
}

func (a *TestMissingTokenAuthenticator) GetTokenFromRequest(r *http.Request) (string, error) {
	return "", ErrNoAuthInfo
}

// PostProcessAuthenticator allows to manipulate or additionally validate authenticated user before returning it.
type PostProcessAuthenticator struct {
	auther   Authenticator
	postFunc func(*models.User) (*models.User, error)
}

func NewPostProcessAuthenticator(auther Authenticator, postFunc func(*models.User) (*models.User, error)) *PostProcessAuthenticator {
	return &PostProcessAuthenticator{auther, postFunc}
}

func (a *PostProcessAuthenticator) Authenticate(token, ip string) (*models.User, error) {
	user, err := a.auther.Authenticate(token, ip)
	if err != nil {
		return nil, err
	}
	return a.postFunc(user)
}

func (a *PostProcessAuthenticator) GetTokenFromRequest(r *http.Request) (string, error) {
	return a.auther.GetTokenFromRequest(r)
}
