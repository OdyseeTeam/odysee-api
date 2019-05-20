package lbrynet

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/proxy"

	ljsonrpc "github.com/lbryio/lbry.go/extras/jsonrpc"
)

const accountNameTemplate string = "lbrytv#user#%s"

// Client is a LBRY SDK jsonrpc client instance
var Client *ljsonrpc.Client

func init() {
	Client = ljsonrpc.NewClient(config.Settings.GetString("Lbrynet"))
}

func accountIDFromEmail(email string) string {
	return fmt.Sprintf(accountNameTemplate, email)
}

// SingleAccountList calls SDK's account_list with accountID as an argument.
// lbry.go's method cannot be used here because
// a) it doesn't support arguments
// b) it doesn't process singular acount_list response
// TODO: Move this method to lbry.go client
func SingleAccountList(email string) (*SingleAccountListResponse, error) {
	accountID := accountIDFromEmail(email)
	response := new(SingleAccountListResponse)
	r, err := proxy.RawCall(proxy.NewRequest("account_list", map[string]string{account_id: accountID}))
	if err != nil {
		return nil, err
	}
	if r.Error.Message != nil {
		if strings.HasPrefix(r.Error, "Couldn't find account:") {
			return nil, AccountNotFound{Email: accountID}
		}
		return nil, errors.New("error in daemon: %v", r.Error.Message)
	}

	err = Decode(r.Result, response)
	if err != nil {
		return nil, err
	}
	return response, err
}

// AccountExists checks if account exists at the local SDK instance
func AccountExists(accountID string) bool {
	a, err := SingleAccountList(accountID)
	if err != nil {
		switch err := err.(type) {
		case AccountNotFound:
			return false
		default:
			// If the daemon errored in any way, safe to assume we shouldn't consider this account non-existing
			return true
		}
	}
}

// CreateAccount creates a new account with the SDK
func CreateAccount(email string) (*ljsonrpc.AccountCreateResponse, error) {
	accountID := accountIDFromEmail(email)
	if AccountExists(email) {
		return nil, AccountConflict{Email: email}
	}
	r, err := Client.AccountCreate(accountName, true)
	if err != nil {
		return nil, err
	}
	return r, nil
}
