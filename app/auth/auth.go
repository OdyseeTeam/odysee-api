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

type Result struct {
	User *models.User
	Err  error
}

func (r *Result) AuthAttempted() bool { return r.User != nil || r.Err != nil }
func (r *Result) AuthFailed() bool    { return r.Err != nil }
func (r *Result) Authenticated() bool { return r.User != nil }

func FromRequest(r *http.Request) *Result {
	v := r.Context().Value(ContextKey)
	if v == nil {
		panic("Auth middleware was not applied")
	}
	return v.(*Result)
}

// Retriever gets a user by hitting internal-api with the provided auth token
// and matching the response to a local user.
// NOTE: The retrieved user must come with a wallet that's created and ready to use.
type Retriever func(token, metaRemoteIP string) (*models.User, error)

func AllInOneRetrieverThatNeedsRefactoring(rt *sdkrouter.Router, internalAPIHost string) Retriever {
	return func(token, metaRemoteIP string) (user *models.User, err error) {
		return wallet.GetUserWithWallet(rt, internalAPIHost, token, metaRemoteIP)
	}
}

func Middleware(retriever Retriever) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ar := &Result{}
			if token, ok := r.Header[wallet.TokenHeader]; ok {
				addr := ip.AddressForRequest(r)
				user, err := retriever(token[0], addr)
				if err != nil {
					logger.LogF(monitor.F{"ip": addr}).Debugf("failed to authenticate user")
					ar.Err = err
				} else {
					ar.User = user
				}
			}
			next.ServeHTTP(w, r.Clone(context.WithValue(r.Context(), ContextKey, ar)))
		})
	}
}
