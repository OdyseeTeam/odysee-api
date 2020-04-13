package users

import (
	"errors"

	"github.com/lbryio/lbrytv/models"
)

func DummyRetriever(userToken, walletID string) UserRetriever {
	return func(token, ip string) (*models.User, error) {
		if userToken == "" || userToken == token {
			return &models.User{WalletID: walletID}, nil
		}
		return nil, errors.New(GenericRetrievalErr)
	}
}
