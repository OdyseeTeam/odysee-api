package auth

import (
	"context"
	"net/http"

	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/internal/ip"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/models"

	"github.com/gorilla/mux"
)

var logger = monitor.NewModuleLogger("auth")

const ContextKey = "user"

func FromRequest(r *http.Request) Result {
	v := r.Context().Value(ContextKey)
	if v == nil {
		panic("Auth middleware was not applied")
	}
	return v.(Result)
}

// Provider gets a user by hitting internal-api with the provided auth token
// and matching the response to a local user.
// NOTE: The retrieved user must come with a wallet that's created and ready to use.
type Provider func(token, metaRemoteIP string) Result

func WalletAndInternalAPIProvider(rt *sdkrouter.Router, internalAPIHost string) Provider {
	return func(token, metaRemoteIP string) Result {
		user, err := wallet.GetUserWithWallet(rt, internalAPIHost, token, metaRemoteIP)
		res := NewResult(user, err)
		if err == nil && user != nil && !user.LbrynetServerID.IsZero() && user.R != nil && user.R.LbrynetServer != nil {
			res.SDKAddress = user.R.LbrynetServer.Address
		}
		return res
	}
}

func Middleware(provider Provider) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var res Result
			if token, ok := r.Header[wallet.TokenHeader]; ok {
				addr := ip.AddressForRequest(r)
				res = provider(token[0], addr)
				if res.err != nil {
					logger.LogF(monitor.F{"ip": addr}).Debugf("error authenticating user")
				}
				res.authAttempted = true
			}
			next.ServeHTTP(w, r.Clone(context.WithValue(r.Context(), ContextKey, res)))
		})
	}
}

// wish i could make this non-exported, but then you can't create new providers outside the package
// don't make this struct directly. instead use NewResult
type Result struct {
	SDKAddress string

	user          *models.User
	err           error
	authAttempted bool
}

func NewResult(user *models.User, err error) Result {
	if err != nil {
		user = nil // err and user cannot be non-nil at the same time
	}
	return Result{user: user, err: err}
}

func (r Result) AuthAttempted() bool {
	return r.authAttempted
}

func (r Result) Authenticated() bool {
	return r.user != nil
}

func (r Result) User() *models.User {
	if !r.authAttempted {
		return nil
	}
	return r.user
}
func (r Result) Err() error {
	if !r.authAttempted {
		return nil
	}
	return r.err
}
