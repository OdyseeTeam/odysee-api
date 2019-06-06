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

// AccountNameFromUID formats the UID for use with SDK.
// UID can be an email or an empty string, in which case a random identifier will be generated.
func AccountNameFromUID(UID string) string {
	if UID != "" {
		return fmt.Sprintf(accountNameTemplate, UID)
	}
	return fmt.Sprintf(accountNameTemplate, crypto.RandString(32))
}

// GetAccount finds account in account_list by UID
func GetAccount(UID string) (*ljsonrpc.Account, error) {
	accounts, err := Client.AccountList()
	if err != nil {
		return nil, err
	}
	for _, account := range accounts.LBCMainnet {
		accountUID := strings.TrimPrefix(account.Name, accountNamePrefix)
		if accountUID == UID {
			return &account, nil
		}
	}
	return nil, AccountNotFound{Email: UID}
}

// CreateAccount creates a new account with the SDK.
// Will return an error if account with this UID already exists.
func CreateAccount(UID string) (*ljsonrpc.AccountCreateResponse, error) {
	accountName := AccountNameFromUID(UID)
	account, err := GetAccount(UID)
	if err == nil {
		logger.LogF(monitor.F{"uid": UID, "account_id": account.ID}).Error("account is already registered with lbrynet")
		return nil, AccountConflict{Email: UID}
	}
	r, err := Client.AccountCreate(accountName, false)
	if err != nil {
		return nil, err
	}
	logger.LogF(monitor.F{"uid": UID, "account_id": r.ID}).Info("registered a new account with lbrynet")
	return r, nil
}

// RemoveAccount removes an account from the SDK by uid
func RemoveAccount(UID string) (*ljsonrpc.AccountRemoveResponse, error) {
	acc, err := GetAccount(UID)
	if err != nil {
		return nil, err
	}
	logger.Log().Infof("removing account %v", UID)
	r, err := Client.AccountRemove(acc.ID)
	if err != nil {
		return nil, err
	}
	return r, nil
}
