package auth

import (
	"context"
	"net/http"

	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/internal/errors"
	"github.com/lbryio/lbrytv/internal/ip"
	"github.com/lbryio/lbrytv/models"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// Middleware tries to authenticate user using request header
func Middleware(provider Provider) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var user *models.User
			var err error
			if token, ok := r.Header[wallet.AuthorizationHeader]; ok {
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

// LegacyMiddleware tries to authenticate user using request header
func LegacyMiddleware(provider Provider) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := FromRequest(r)
			if user == nil && err != nil {
				if token, ok := r.Header[wallet.TokenHeader]; ok {
					addr := ip.FromRequest(r)
					user, err = provider(token[0], addr)
					if err != nil {
						logger.WithFields(logrus.Fields{"ip": addr}).Debugf("error authenticating user")
					}
				} else {
					err = errors.Err(ErrNoAuthInfo)
				}
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
	return LegacyMiddleware(p)
}
