package auth

import (
	"context"
	"net/http"
	"strconv"

	"github.com/OdyseeTeam/odysee-api/app/sdkrouter"
	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/internal/errors"
	"github.com/OdyseeTeam/odysee-api/internal/ip"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/getsentry/sentry-go"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// Middleware tries to authenticate user using request header
func Middleware(auther Authenticator) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var user *models.User
			token, err := auther.GetTokenFromHeader(r.Header)
			if err != nil {
				logger.Log().Debugf("cannot retrieve token from request: %s", err)
				next.ServeHTTP(w, r.Clone(context.WithValue(r.Context(), contextKey, result{user, err})))
				return
			}
			addr := ip.FromRequest(r)
			user, err = auther.Authenticate(token, addr)
			if err != nil {
				logger.WithFields(logrus.Fields{"ip": addr}).Debugf("error authenticating user: %s", err)
			}
			if user != nil {
				if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
					hub.Scope().SetUser(sentry.User{ID: strconv.Itoa(user.ID), IPAddress: addr})
				}
			}
			next.ServeHTTP(w, r.Clone(context.WithValue(r.Context(), contextKey, result{user, err})))
		})
	}
}

// LegacyMiddleware tries to authenticate user using request header
func LegacyMiddleware(provider Provider) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			addr := ip.FromRequest(r)
			user, err := FromRequest(r)
			if user == nil && err != nil {
				if token, ok := r.Header[wallet.LegacyTokenHeader]; ok {
					user, err = provider(token[0], addr)
					if err != nil {
						logger.WithFields(logrus.Fields{"ip": addr}).Debugf("error authenticating user")
					}
				} else {
					err = errors.Err(wallet.ErrNoAuthInfo)
				}
			}
			if user != nil {
				if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
					hub.Scope().SetUser(sentry.User{ID: strconv.Itoa(user.ID), IPAddress: addr})
				}
			}

			next.ServeHTTP(w, r.Clone(context.WithValue(r.Context(), contextKey, result{user, err})))
		})
	}
}

// NilMiddleware is useful when you need to test your logic without involving real authentication
var NilMiddleware = LegacyMiddleware(nilProvider)

// MiddlewareWithProvider is useful when you want to
func MiddlewareWithProvider(rt *sdkrouter.Router, internalAPIHost string) mux.MiddlewareFunc {
	p := NewIAPIProvider(rt, internalAPIHost)
	return LegacyMiddleware(p)
}
