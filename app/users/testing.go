package users

import (
	"errors"

	"github.com/lbryio/lbrytv/models"
)

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
