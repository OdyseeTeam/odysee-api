package auth

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/internal/errors"
	"github.com/lbryio/lbrytv/internal/ip"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/models"
	"github.com/sirupsen/logrus"
)

var (
	logger      = monitor.NewModuleLogger("auth")
	nilProvider = func(token, ip string) (*models.User, error) { return nil, nil }

	ErrNoAuthInfo = errors.Base("authentication token missing")
)

const contextKey ctxKey = iota

type ctxKey int

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

// Middleware tries to authenticate user using request header
func Middleware(provider Provider) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var user *models.User
			var err error
			if token, ok := r.Header[wallet.TokenHeader]; ok {
				addr := ip.FromRequest(r)
				user, err = provider(token[0], addr)
				if err != nil {
					logger.WithFields(logrus.Fields{"ip": addr}).Debugf("error authenticating user")
				}
			} else {
				err = errors.Err(ErrNoAuthInfo)
			}
			next.ServeHTTP(w, r.Clone(context.WithValue(r.Context(), contextKey, result{user, err})))
		})
	}
}

// NilMiddleware is useful when you need to test your logic without involving real authentication
var NilMiddleware = Middleware(nilProvider)

// MiddlewareWithProvider is useful when you want to
func MiddlewareWithProvider(rt *sdkrouter.Router, internalAPIHost string) mux.MiddlewareFunc {
	p := NewIAPIProvider(rt, internalAPIHost)
	return Middleware(p)
}
