package lbrynet

import (
	"errors"
	"fmt"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/monitor"

	ljsonrpc "github.com/lbryio/lbry.go/extras/jsonrpc"
)

const accountNamePrefix string = "lbry#user#id:"
const accountNameTemplate string = accountNamePrefix + "%v"

// Client is a LBRY SDK jsonrpc client instance
var Client = ljsonrpc.NewClient(config.Settings.GetString("Lbrynet"))

var logger = monitor.NewModuleLogger("lbrynet")

// MakeAccountName formats the UID for use with SDK.
// UID can be an email or an empty string, in which case a random identifier will be generated.
func MakeAccountName(UID int) string {
	return fmt.Sprintf(accountNameTemplate, UID)
}

// GetAccount finds account in account_list by UID
func GetAccount(UID int) (*ljsonrpc.Account, error) {
	requiredAccountName := MakeAccountName(UID)
	accounts, err := Client.AccountList()
	if err != nil {
		return nil, err
	}
	for _, account := range accounts.LBCMainnet {
		if account.Name == requiredAccountName {
			return &account, nil
		}
	}
	return nil, AccountNotFound{UID: UID}
}

// CreateAccount creates a new account with the SDK.
// Will return an error if account with this UID already exists.
func CreateAccount(UID int) (*ljsonrpc.AccountCreateResponse, error) {
	accountName := MakeAccountName(UID)
	account, err := GetAccount(UID)
	if err == nil {
		logger.LogF(monitor.F{"uid": UID, "account_id": account.ID}).Error("account is already registered with lbrynet")
		return nil, AccountConflict{UID: UID}
	}
	r, err := Client.AccountCreate(accountName, false)
	if err != nil {
		return nil, err
	}
	logger.LogF(monitor.F{"uid": UID, "account_id": r.ID}).Info("registered a new account with lbrynet")
	return r, nil
}

// RemoveAccount removes an account from the SDK by uid
func RemoveAccount(UID int) (*ljsonrpc.AccountRemoveResponse, error) {
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

// Resolve calls resolve method on the daemon and handles
// *frequent* SDK response format changes with grace instead of panicking.
func Resolve(url string) (*ljsonrpc.ResolveResponseItem, error) {
	r, err := Client.Resolve(url)
	if err != nil {
		return nil, err
	}
	item := (*r)[url]

	// TODO: Change when underlying libs are updated for 0.38
	if item.Claim == nil {
		return nil, errors.New("invalid resolve response structure from sdk client")
	}
	return &item, nil
}
