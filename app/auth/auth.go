package auth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/OdyseeTeam/odysee-api/app/sdkrouter"
	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/internal/errors"
	"github.com/OdyseeTeam/odysee-api/internal/monitor"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/iapi"
)

var (
	logger      = monitor.NewModuleLogger("auth")
	nilProvider = func(token, ip string) (*models.User, error) { return nil, nil }
)

type ctxKey int

const userContextKey ctxKey = iota

type CurrentUser struct {
	IP         string
	IAPIClient *iapi.Client

	user *models.User
	err  error
}

type IAPIUserClient interface {
}

type Authenticator interface {
	Authenticate(token, metaRemoteIP string) (*models.User, error)
	GetTokenFromRequest(r *http.Request) (string, error)
}

// Provider tries to authenticate using the provided auth token
type Provider func(token, metaRemoteIP string) (*models.User, error)

// FromRequest retrieves user from http.Request that went through our Middleware
func FromRequest(r *http.Request) (*models.User, error) {
	cu, err := GetCurrentUserData(r.Context())
	if err != nil {
		return nil, err
	}
	return cu.user, cu.err
}

func AttachCurrentUser(ctx context.Context, cu *CurrentUser) context.Context {
	return context.WithValue(ctx, userContextKey, cu)
}

// GetCurrentUserData retrieves user from http.Request that went through our Middleware
func GetCurrentUserData(ctx context.Context) (*CurrentUser, error) {
	v := ctx.Value(userContextKey)
	if v == nil {
		return nil, errors.Err("auth middleware is required")
	}
	res := v.(*CurrentUser)
	if res == nil {
		return nil, fmt.Errorf("%v is not CurrentUser", v)
	}
	if res.err != nil {
		return nil, res.err
	}
	return res, nil
}

func NewCurrentUser(u *models.User, e error) *CurrentUser {
	return &CurrentUser{user: u, err: e}
}

func (cu CurrentUser) User() *models.User {
	return cu.user
}

func (cu CurrentUser) Err() error {
	return cu.err
}

// NewIAPIProvider authenticates a user by hitting internal-api with the auth token
// and matching the response to a local user. If auth is successful, the user will have a
// lbrynet server assigned and a wallet that's created and ready to use.
func NewIAPIProvider(router *sdkrouter.Router, internalAPIHost string) Provider {
	return func(token, metaRemoteIP string) (*models.User, error) {
		return wallet.GetUserWithSDKServer(router, internalAPIHost, token, metaRemoteIP)
	}
}

// NewOauthProvider authenticates a user by validating the access token passed in the
// authorization header. If the keycloak user id is stored locally then that user will
// be returned. If not then we reach out to internal-apis to get the internal-apis
// user id and use that to create the known wallet id, save it along with the user id
// to the user in question. If auth is successful, the user will have a
// lbrynet server assigned and a wallet that's created and ready to use.
func NewOauthProvider(oauthProviderURL string, clientID string, iapiURL string, router *sdkrouter.Router) Provider {
	auther, err := wallet.NewOauthAuthenticator(oauthProviderURL, clientID, iapiURL, router)
	if err != nil {
		panic(err)
	}
	return func(token, metaRemoteIP string) (*models.User, error) {
		return auther.Authenticate(token, metaRemoteIP)
	}
}
