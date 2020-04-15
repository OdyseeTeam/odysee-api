package users

import (
	"net/http"

	"github.com/lbryio/lbrytv/internal/monitor"
)

const GenericRetrievalErr = "unable to retrieve user"

var logger = monitor.NewModuleLogger("auth")

type Authenticator struct {
	retriever Retriever
}

type AuthenticatedRequest struct {
	*http.Request
	WalletID  string
	AuthError error
}

type AuthenticatedFunc func(http.ResponseWriter, *AuthenticatedRequest)

// NewAuthenticator provides HTTP handler wrapping methods
// and should be initialized with an object that allows user retrieval.
func NewAuthenticator(retriever Retriever) *Authenticator {
	return &Authenticator{retriever}
}

// GetWalletID retrieves user token from HTTP headers and subsequently
// an SDK account ID from Retriever.
func (a *Authenticator) GetWalletID(r *http.Request) (string, error) {
	if token, ok := r.Header[TokenHeader]; ok {
		ip := GetIPAddressForRequest(r)
		u, err := a.retriever.Retrieve(Query{Token: token[0], MetaRemoteIP: ip})
		log := logger.LogF(monitor.F{"ip": ip})
		if err != nil {
			logger.LogF(monitor.F{"ip": ip}).Errorf("failed to authenticate user: %v", err)
			return "", err
		} else if u != nil {
			logger.LogF(monitor.F{"ip": ip, "user_id": u.ID}).Infof("user authenticated")
			return u.WalletID, nil
		}
	}
	return "", nil
}

// Wrap result can be supplied to all functions that accept http.HandleFunc,
// supplied function will be wrapped and called with AuthenticatedRequest instead of http.Request.
func (a *Authenticator) Wrap(wrapped AuthenticatedFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		WalletID, err := a.GetWalletID(r)
		ar := &AuthenticatedRequest{Request: r}
		if err != nil {
			ar.AuthError = err
		} else {
			ar.WalletID = WalletID
		}
		wrapped(w, ar)
	}
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
