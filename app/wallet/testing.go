package wallet

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/lbryio/lbrytv/apps/lbrytv/config"
	"github.com/lbryio/lbrytv/models"
	"github.com/volatiletech/null"

	"golang.org/x/oauth2"
)

const (
	TestClientID = "ci-tester"

	envClientSecret = "OAUTH_TEST_CLIENT_SECRET"
	envUsername     = "OAUTH_TEST_USERNAME"
	envPassword     = "OAUTH_TEST_PASSWORD"

	msgMissingEnv = "test oauth client env var %s is not set"
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

// GetTestToken is for easily retrieving tokens that can be used in tests utilizing authentication subsystem.
func GetTestToken() (*oauth2.Token, error) {
	clientSecret := os.Getenv(envClientSecret)
	username := os.Getenv(envUsername)
	password := os.Getenv(envPassword)
	if clientSecret == "" {
		return nil, fmt.Errorf(msgMissingEnv, envClientSecret)
	}
	if username == "" {
		return nil, fmt.Errorf(msgMissingEnv, envUsername)
	}
	if password == "" {
		return nil, fmt.Errorf(msgMissingEnv, envPassword)
	}

	ctx := context.Background()
	conf := &oauth2.Config{
		// ClientID:     config.GetOauthClientID(),
		ClientID:     TestClientID,
		ClientSecret: clientSecret,
		Endpoint:     oauth2.Endpoint{TokenURL: config.GetOauthTokenURL()},
	}
	return conf.PasswordCredentialsToken(ctx, username, password)
}

func GetTestTokenHeader() (string, error) {
	t, err := GetTestToken()
	if err != nil {
		return "", err
	}
	return TokenPrefix + t.AccessToken, nil
}
