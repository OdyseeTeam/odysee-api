package wallet

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/OdyseeTeam/odysee-api/app/sdkrouter"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/sirupsen/logrus"

	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
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

func CreateTestUser(rt *sdkrouter.Router, exec boil.ContextBeginner, id int) (*models.User, error) {
	var localUser *models.User

	err := inTx(context.Background(), exec, func(tx *sql.Tx) error {
		l := logrus.WithFields(logrus.Fields{})
		localUser, err := getOrCreateLocalUser(tx, models.User{ID: id}, l)
		if err != nil {
			return err
		}

		if localUser.LbrynetServerID.IsZero() {
			err := assignSDKServerToUser(tx, localUser, rt.LeastLoaded(), l)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return localUser, err
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
