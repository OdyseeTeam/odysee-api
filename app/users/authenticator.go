package users

import (
	"errors"
	"net/http"
)

const GenericRetrievalErr = "unable to retrieve user"

type Authenticator struct {
	retriever Retriever
}

type AuthenticatedRequest struct {
	*http.Request
	AccountID string
	AuthError error
}

type AuthenticatedFunc func(http.ResponseWriter, *AuthenticatedRequest)

// NewAuthenticator provides HTTP handler wrapping methods
// and should be initialized with an object that allows user retrieval.
func NewAuthenticator(retriever Retriever) *Authenticator {
	return &Authenticator{retriever}
}

// GetAccountID retrieves user token from HTTP headers and subsequently
// an SDK account ID from Retriever.
func (a *Authenticator) GetAccountID(r *http.Request) (string, error) {
	if token, ok := r.Header[TokenHeader]; ok {
		u, err := a.retriever.Retrieve(Query{token[0], GetIPAddressForRequest(r)})
		if err != nil {
			return "", err
		}
		if u == nil {
			return "", errors.New(GenericRetrievalErr)
		}
		return u.SDKAccountID, nil
	}
	return "", nil
}

// Wrap result can be supplied to all functions that accept http.HandleFunc,
// supplied function will be wrapped and called with AuthenticatedRequest instead of http.Request.
func (a *Authenticator) Wrap(wrapped AuthenticatedFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		AccountID, err := a.GetAccountID(r)
		ar := &AuthenticatedRequest{Request: r}
		if err != nil {
			ar.AuthError = err
		} else {
			ar.AccountID = AccountID
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
	return r.AccountID != ""
}
