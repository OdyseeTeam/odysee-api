package lbrynet

import (
	"fmt"
	"strings"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/monitor"

	ljsonrpc "github.com/lbryio/lbry.go/extras/jsonrpc"
)

const accountNamePrefix string = "lbrytv#user#"
const accountNameTemplate string = accountNamePrefix + "%s"

// Client is a LBRY SDK jsonrpc client instance
var Client = ljsonrpc.NewClient(config.Settings.GetString("Lbrynet"))

var logger = monitor.NewModuleLogger("lbrynet")

// AccountNameFromEmail returns email formatted for internal use
func AccountNameFromEmail(email string) string {
	return fmt.Sprintf(accountNameTemplate, email)
}

// GetAccount finds account in account_list by email
func GetAccount(email string) (*ljsonrpc.Account, error) {
	accounts, err := Client.AccountList()
	if err != nil {
		return nil, err
	}
	for _, account := range accounts.LBCMainnet {
		accountEmail := strings.TrimPrefix(account.Name, accountNamePrefix)
		if accountEmail == email {
			return &account, nil
		}
	}
	return nil, AccountNotFound{Email: email}
}

// AccountExists checks if account exists at the local SDK instance.
// In case of any errors apart from AccountNotFound we like want to break the flow of the caller and return true.
func AccountExists(email string) bool {
	_, err := GetAccount(email)
	if err != nil {
		switch err.(type) {
		case AccountNotFound:
			return false
		default:
			return true
		}
	}
	return true
}

// CreateAccount creates a new account with the SDK
func CreateAccount(email string) (*ljsonrpc.AccountCreateResponse, error) {
	accountName := AccountNameFromEmail(email)
	if AccountExists(email) {
		logger.Log().Errorf("email %v is registered with the daemon", email)
		return nil, AccountConflict{Email: email}
	}
	logger.Log().Infof("creating account %v", email)
	r, err := Client.AccountCreate(accountName, false)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// RemoveAccount removes an account from the SDK by email
func RemoveAccount(email string) (*ljsonrpc.AccountRemoveResponse, error) {
	acc, err := GetAccount(email)
	if err != nil {
		return nil, err
	}
	logger.Log().Infof("removing account %v", email)
	r, err := Client.AccountRemove(acc.ID)
	if err != nil {
		return nil, err
	}
	return r, nil
}
