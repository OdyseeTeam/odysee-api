package users

import (
	"net/http"

	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/models"
)

const GenericRetrievalErr = "unable to retrieve user"

var logger = monitor.NewModuleLogger("auth")

type AuthenticatedRequest struct {
	*http.Request
	WalletID  string
	AuthError error
}

// AuthFailed is a helper to see if there was an error authenticating user.
func (r *AuthenticatedRequest) AuthFailed() bool {
	return r.AuthError != nil
}

// IsAuthenticated is a helper to see if a user was authenticated.
// If it is false, AuthError might be provided (in case user retriever has errored)
// or be nil if no auth token was present in headers.
func (r *AuthenticatedRequest) IsAuthenticated() bool {
	return r.WalletID != ""
}

// Retriever is an interface for user retrieval by internal-apis auth token
type UserRetriever func(token, metaRemoteIP string) (*models.User, error)

type Authenticator struct {
	Retriever UserRetriever
}

// NewAuthenticator provides HTTP handler wrapping methods
// and should be initialized with an object that allows user retrieval.
func NewAuthenticator(rt *sdkrouter.Router) *Authenticator {
	return &Authenticator{
		Retriever: func(token, metaRemoteIP string) (user *models.User, err error) {
			return wallet.GetUserWithWallet(rt, token, metaRemoteIP)
		},
	}
}

// GetWalletID retrieves user token from HTTP headers and subsequently
// an SDK account ID from Retriever.
func (a *Authenticator) GetWalletID(r *http.Request) (string, error) {
	if token, ok := r.Header[wallet.TokenHeader]; ok {
		ip := GetIPAddressForRequest(r)
		user, err := a.Retriever(token[0], ip)
		if err != nil {
			logger.LogF(monitor.F{"ip": ip}).Debugf("failed to authenticate user")
			return "", err
		} else if user != nil {
			return user.WalletID, nil
		}
	}
	return "", nil
}

// Wrap result can be supplied to all functions that accept http.HandleFunc,
// supplied function will be wrapped and called with AuthenticatedRequest instead of http.Request.
func (a *Authenticator) Wrap(wrapped func(http.ResponseWriter, *AuthenticatedRequest)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ar := &AuthenticatedRequest{Request: r}
		WalletID, err := a.GetWalletID(r)
		if err != nil {
			ar.AuthError = err
		} else {
			ar.WalletID = WalletID
		}
		wrapped(w, ar)
	}
}
