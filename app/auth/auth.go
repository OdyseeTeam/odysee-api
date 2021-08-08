package auth

import (
	"net/http"

	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/internal/errors"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/models"
)

var (
	logger      = monitor.NewModuleLogger("auth")
	nilProvider = func(token, ip string) (*models.User, error) { return nil, nil }

	ErrNoAuthInfo = errors.Base("authentication token missing")
)

type ctxKey int

const contextKey ctxKey = iota

type result struct {
	user *models.User
	err  error
}

// FromRequest retrieves user from http.Request that went through our Middleware
func FromRequest(r *http.Request) (*models.User, error) {
	v := r.Context().Value(contextKey)
	if v == nil {
		return nil, errors.Err("auth.Middleware is required")
	}
	res := v.(result)
	return res.user, res.err
}

// Provider tries to authenticate using the provided auth token
type Provider func(token, metaRemoteIP string) (*models.User, error)

// NewIAPIProvider authenticates a user by hitting internal-api with the auth token
// and matching the response to a local user. If auth is successful, the user will have a
// lbrynet server assigned and a wallet that's created and ready to use.
func NewIAPIProvider(rt *sdkrouter.Router, internalAPIHost string) Provider {
	return func(token, metaRemoteIP string) (*models.User, error) {
		return wallet.GetUserWithSDKServer(rt, internalAPIHost, token, metaRemoteIP)
	}
}

// NewOauthProvider authenticates a user by validating the access token passed in the
// authorization header. If the keycloak user id is stored locally then that user will
// be returned. If not then we reach out to internal-apis to get the internal-apis
// user id and use that to create the known wallet id, save it along with the user id
// to the user in question. If auth is successful, the user will have a
// lbrynet server assigned and a wallet that's created and ready to use.
func NewOauthProvider(rt *sdkrouter.Router, internalAPIHost string) Provider {
	return func(token, metaRemoteIP string) (*models.User, error) {
		return wallet.GetOauthUserWithSDKServer(rt, internalAPIHost, token, metaRemoteIP)
	}
}
