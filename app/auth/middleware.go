package auth

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/OdyseeTeam/odysee-api/app/sdkrouter"
	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/internal/ip"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/iapi"
	"github.com/getsentry/sentry-go"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// Middleware tries to authenticate user using request header
func Middleware(auther Authenticator) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var user *models.User
			var iac *iapi.Client
			ipAddr := ip.FromRequest(r)
			token, err := auther.GetTokenFromRequest(r)
			if err != nil {
				logger.Log().Debugf("cannot retrieve token from request: %s", err)
				next.ServeHTTP(w, r.Clone(context.WithValue(r.Context(), userContextKey, &CurrentUser{err: err})))
				return
			}
			user, err = auther.Authenticate(token, ipAddr)
			if err != nil {
				logger.WithFields(logrus.Fields{"ip": ipAddr}).Debugf("error authenticating user: %s", err)
			}
			if user != nil {
				iac, _ = iapi.NewClient(
					iapi.WithOAuthToken(strings.TrimPrefix(token, wallet.TokenPrefix)),
					iapi.WithRemoteIP(ipAddr),
				)
				if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
					hub.Scope().SetUser(sentry.User{ID: strconv.Itoa(user.ID), IPAddress: ipAddr})
				}
			}

			cu := NewCurrentUser(user, err)
			cu.IP = ipAddr
			cu.IAPIClient = iac
			next.ServeHTTP(w, r.Clone(context.WithValue(r.Context(), userContextKey, cu)))
		})
	}
}

// LegacyMiddleware tries to authenticate user using request header
func LegacyMiddleware(provider Provider) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var err error
			var iac *iapi.Client
			ipAddr := ip.FromRequest(r)
			user, err := FromRequest(r)
			if err != nil {
				token := r.Header.Get(wallet.LegacyTokenHeader)
				if token != "" {
					user, err = provider(token, ipAddr)
					if err != nil {
						logger.WithFields(logrus.Fields{"ip": ipAddr}).Debugf("error authenticating user")
					}
				} else {
					err = wallet.ErrNoAuthInfo
				}
				if user != nil {
					iac, _ = iapi.NewClient(
						iapi.WithLegacyToken(token),
						iapi.WithRemoteIP(ipAddr),
					)
					if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
						hub.Scope().SetUser(sentry.User{ID: strconv.Itoa(user.ID), IPAddress: ipAddr})
					}
				}
				cu := NewCurrentUser(user, err)
				cu.IP = ipAddr
				cu.IAPIClient = iac
				next.ServeHTTP(w, r.Clone(context.WithValue(r.Context(), userContextKey, cu)))
				return
			}

			next.ServeHTTP(w, r)
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
