package users

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/lbryio/lbrytv/internal/responses"
	"github.com/lbryio/lbrytv/models"
)

const userHasVerifiedEmailResponse = `{
	"success": true,
	"error": null,
	"data": {
	  "user_id": %v,
	  "has_verified_email": true
	}
}`

const userDoesntHaveVerifiedEmailResponse = `{
	"success": true,
	"error": null,
	"data": {
	  "user_id": %v,
	  "has_verified_email": false
	}
}`

// TestUserRetriever is a helper allowing to test API endpoints that require authentication
// without actually creating DB records.
type TestUserRetriever struct {
	WalletID string
	Token    string
}

// Retrieve returns WalletID set during TestUserRetriever creation,
// checking it against TestUserRetriever's Token field if one was supplied.
func (r *TestUserRetriever) Retrieve(q Query) (*models.User, error) {
	if r.Token == "" || r.Token == q.Token {
		return &models.User{WalletID: r.WalletID}, nil
	}
	return nil, errors.New(GenericRetrievalErr)
}

func StartDummyAPIServer(response []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responses.PrepareJSONWriter(w)
		w.Write(response)
	}))
}

func StartAuthenticatingAPIServer(userID int) *httptest.Server {
	response := fmt.Sprintf(userHasVerifiedEmailResponse, userID)
	return StartDummyAPIServer([]byte(response))
}

func StartEasyAPIServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := r.PostFormValue("auth_token")
		reply := fmt.Sprintf(userHasVerifiedEmailResponse, t)
		responses.PrepareJSONWriter(w)
		w.Write([]byte(reply))
	}))
}
