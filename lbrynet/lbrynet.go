package lbrynet

import (
	"fmt"
	"strings"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/monitor"

	"github.com/lbryio/lbry.go/extras/crypto"
	ljsonrpc "github.com/lbryio/lbry.go/extras/jsonrpc"
)

const accountNamePrefix string = "lbrytv#user#"
const accountNameTemplate string = accountNamePrefix + "%s"

// Client is a LBRY SDK jsonrpc client instance
var Client = ljsonrpc.NewClient(config.Settings.GetString("Lbrynet"))

var logger = monitor.NewModuleLogger("lbrynet")

// AccountNameFromUID returns uid formatted for internal use
func AccountNameFromUID(uid string) string {
	if uid != "" {
		return fmt.Sprintf(accountNameTemplate, uid)
	}
	return fmt.Sprintf(accountNameTemplate, crypto.RandString(32))
}

// GetAccount finds account in account_list by uid
func GetAccount(uid string) (*ljsonrpc.Account, error) {
	accounts, err := Client.AccountList()
	if err != nil {
		return nil, err
	}
	for _, account := range accounts.LBCMainnet {
		accountUID := strings.TrimPrefix(account.Name, accountNamePrefix)
		if accountUID == uid {
			return &account, nil
		}
	}
	return nil, AccountNotFound{Email: uid}
}

// AccountExists checks if account exists at the local SDK instance.
// In case of any errors apart from AccountNotFound we like want to break the flow of the caller and return true.
func AccountExists(uid string) bool {
	_, err := GetAccount(uid)
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
func CreateAccount(uid string) (*ljsonrpc.AccountCreateResponse, error) {
	accountName := AccountNameFromUID(uid)
	if uid != "" && AccountExists(uid) {
		logger.Log().Errorf("uid %v is registered with the daemon", uid)
		return nil, AccountConflict{Email: uid}
	}
	logger.Log().Infof("creating account %v", uid)
	r, err := Client.AccountCreate(accountName, false)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// RemoveAccount removes an account from the SDK by uid
func RemoveAccount(uid string) (*ljsonrpc.AccountRemoveResponse, error) {
	acc, err := GetAccount(uid)
	if err != nil {
		return nil, err
	}
	logger.Log().Infof("removing account %v", uid)
	r, err := Client.AccountRemove(acc.ID)
	if err != nil {
		return nil, err
	}
	return r, nil
}
