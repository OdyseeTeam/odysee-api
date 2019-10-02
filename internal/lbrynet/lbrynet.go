package lbrynet

import (
	"errors"
	"fmt"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/monitor"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
)

const accountNamePrefix string = "lbrytv-user-id:"
const accountNameTemplate string = accountNamePrefix + "%v"

const walletNameTemplate string = "lbrytv-id.%v.wallet"

var defaultWalletOpts = ljsonrpc.WalletCreateOpts{SkipOnStartup: false, CreateAccount: true, SingleKey: true}

// Client is a LBRY SDK jsonrpc client instance
var Client = ljsonrpc.NewClient(config.GetLbrynet())

var logger = monitor.NewModuleLogger("lbrynet")

// MakeAccountName formats user ID to use as an SDK account name.
func MakeAccountName(uid int) string {
	return fmt.Sprintf(accountNameTemplate, uid)
}

// MakeWalletID formats user ID to use as an SDK wallet ID.
func MakeWalletID(uid int) string {
	return fmt.Sprintf(walletNameTemplate, uid)
}

// GetAccount finds account in account_list by UID
func GetAccount(uid int) (*ljsonrpc.Account, error) {
	requiredAccountName := MakeAccountName(uid)
	accounts, err := Client.AccountList()
	if err != nil {
		return nil, err
	}
	for _, account := range accounts.LBCMainnet {
		if account.Name == requiredAccountName {
			return &account, nil
		}
	}
	return nil, AccountNotFound{UID: uid}
}

// CreateAccount creates a new account with the SDK.
// Will return an error if account with this UID already exists.
func CreateAccount(UID int) (*ljsonrpc.Account, error) {
	accountName := MakeAccountName(UID)
	account, err := GetAccount(UID)
	if err == nil {
		logger.LogF(monitor.F{"uid": UID, "account_id": account.ID}).Error("account is already registered with lbrynet")
		return nil, AccountConflict{UID: UID}
	}
	r, err := Client.AccountCreate(accountName, true)
	if err != nil {
		return nil, err
	}
	logger.LogF(monitor.F{"uid": UID, "account_id": r.ID}).Info("registered a new account with lbrynet")
	return r, nil
}

// RemoveAccount removes an account from the SDK by uid
func RemoveAccount(UID int) (*ljsonrpc.Account, error) {
	acc, err := GetAccount(UID)
	if err != nil {
		return nil, err
	}
	logger.LogF(monitor.F{"uid": UID, "account_id": acc.ID}).Warn("removing account from lbrynet")
	r, err := Client.AccountRemove(acc.ID)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// InitializeWallet creates a wallet that can be immediately used
// in subsequent commands.
// It can recover from errors like existing wallets, but if a wallet is known to exist
// (eg. a wallet ID stored in the database already), AddWallet should be called instead.
func InitializeWallet(uid int) (string, error) {
	wid := MakeWalletID(uid)
	log := logger.LogF(monitor.F{"wallet_id": wid, "user_id": uid})
	wallet, err := CreateWallet(uid)
	if err != nil {
		if errors.As(err, &WalletExists{}) {
			log.Warn(err.Error())
			return wid, nil
		} else if errors.As(err, &WalletNeedsLoading{}) {
			log.Info(err.Error())
			wallet, err = AddWallet(uid)
			if err != nil && errors.As(err, &WalletAlreadyLoaded{}) {
				log.Info(err.Error())
				return wid, nil
			}
		} else {
			log.Error("don't know how to recover from error: ", err)
			return "", err
		}
	}
	return wallet.ID, nil
}

// CreateWallet creates a new wallet with the SDK.
// Returned error doesn't necessarily mean that the wallet is not operational:
//
// 	if errors.Is(err, lbrynet.WalletExists) {
// 	 // Okay to proceed with the account
//  }
//
// 	if errors.Is(err, lbrynet.WalletNeedsLoading) {
// 	 // AddWallet() needs to be called before the wallet can be used
//  }
func CreateWallet(uid int) (*ljsonrpc.Wallet, error) {
	wid := MakeWalletID(uid)
	log := logger.LogF(monitor.F{"wallet_id": wid, "user_id": uid})
	wallet, err := Client.WalletCreate(wid, &defaultWalletOpts)
	if err != nil {
		return nil, NewWalletError(uid, err)
	}
	log.Info("wallet created")
	return wallet, nil
}

// AddWallet loads an existing wallet in the SDK.
// May return errors:
//  WalletAlreadyLoaded - wallet is already loaded and operational
//  WalletNotFound - wallet file does not exist and won't be loaded.
func AddWallet(uid int) (*ljsonrpc.Wallet, error) {
	wid := MakeWalletID(uid)
	log := logger.LogF(monitor.F{"wallet_id": wid, "user_id": uid})
	wallet, err := Client.WalletAdd(wid)
	if err != nil {
		return nil, NewWalletError(uid, err)
	}
	log.Info("wallet loaded")
	return wallet, nil
}

// WalletRemove loads an existing wallet in the SDK.
// May return errors:
//  WalletAlreadyLoaded - wallet is already loaded and operational
//  WalletNotFound - wallet file does not exist and won't be loaded.
func WalletRemove(uid int) (*ljsonrpc.Wallet, error) {
	wid := MakeWalletID(uid)
	log := logger.LogF(monitor.F{"wallet_id": wid, "user_id": uid})
	wallet, err := Client.WalletRemove(wid)
	if err != nil {
		return nil, NewWalletError(uid, err)
	}
	log.Info("wallet removed")
	return wallet, nil
}

// Resolve calls resolve method on the daemon and handles
// *frequent* SDK response format changes with grace instead of panicking.
func Resolve(url string) (*ljsonrpc.Claim, error) {
	r, err := Client.Resolve(url)
	if err != nil {
		return nil, err
	}
	item := (*r)[url]

	if item.CanonicalURL == "" {
		return nil, errors.New("invalid resolve response structure from sdk client")
	}
	return &item, nil
}
