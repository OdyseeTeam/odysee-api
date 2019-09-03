package users

import (
	"errors"

	"github.com/lbryio/lbrytv/models"
)

// TestUserRetriever is a helper allowing to test API endpoints that require authentication
// without actually creating DB records.
type TestUserRetriever struct {
	AccountID string
	Token     string
}

// Retrieve returns AccountID set during TestUserRetriever creation,
// checking it against TestUserRetriever's Token field if one was supplied.
func (r *TestUserRetriever) Retrieve(token string) (*models.User, error) {
	if r.Token == "" || r.Token == token {
		return &models.User{SDKAccountID: r.AccountID}, nil
	}
	return nil, errors.New(GenericRetrievalErr)
}
