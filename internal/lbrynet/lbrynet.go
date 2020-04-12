package lbrynet

import (
	"errors"

	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/internal/monitor"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
)

var Logger = monitor.NewModuleLogger("lbrynet")

// InitializeWallet creates a wallet that can be immediately used in subsequent commands.
// It can recover from errors like existing wallets, but if a wallet is known to exist
// (eg. a wallet ID stored in the database already), loadWallet() should be called instead.
func InitializeWallet(rt *sdkrouter.Router, userID int) (string, error) {
	w, err := createWallet(rt, userID)
	if err == nil {
		return w.ID, nil
	}

	walletID := sdkrouter.WalletID(userID)
	log := Logger.LogF(monitor.F{"user_id": userID})

	if errors.Is(err, ErrWalletExists) {
		log.Warn(err.Error())
		return walletID, nil
	}

	if errors.Is(err, ErrWalletNeedsLoading) {
		log.Info(err.Error())
		w, err = loadWallet(rt, userID)
		if err != nil {
			if errors.Is(err, ErrWalletAlreadyLoaded) {
				log.Info(err.Error())
				return walletID, nil
			}
			return "", err
		}
		return w.ID, nil
	}

	log.Errorf("don't know how to recover from error: %v", err)
	return "", err
}

// createWallet creates a new wallet on the LbrynetServer.
// Returned error doesn't necessarily mean that the wallet is not operational:
//
// 	if errors.Is(err, lbrynet.WalletExists) {
// 	 // Okay to proceed with the account
//  }
//
// 	if errors.Is(err, lbrynet.WalletNeedsLoading) {
// 	 // loadWallet() needs to be called before the wallet can be used
//  }
func createWallet(rt *sdkrouter.Router, userID int) (*ljsonrpc.Wallet, error) {
	wallet, err := rt.Client(userID).WalletCreate(sdkrouter.WalletID(userID), &ljsonrpc.WalletCreateOpts{
		SkipOnStartup: true, CreateAccount: true, SingleKey: true})
	if err != nil {
		return nil, NewWalletError(userID, err)
	}
	Logger.LogF(monitor.F{"user_id": userID}).Info("wallet created")
	return wallet, nil
}

// loadWallet loads an existing wallet in the LbrynetServer.
// May return errors:
//  WalletAlreadyLoaded - wallet is already loaded and operational
//  WalletNotFound - wallet file does not exist and won't be loaded.
func loadWallet(rt *sdkrouter.Router, userID int) (*ljsonrpc.Wallet, error) {
	wallet, err := rt.Client(userID).WalletAdd(sdkrouter.WalletID(userID))
	if err != nil {
		return nil, NewWalletError(userID, err)
	}
	Logger.LogF(monitor.F{"user_id": userID}).Info("wallet loaded")
	return wallet, nil
}

// UnloadWallet unloads an existing wallet from the LbrynetServer.
// May return errors:
//  WalletAlreadyLoaded - wallet is already loaded and operational
//  WalletNotFound - wallet file does not exist and won't be loaded.
func UnloadWallet(rt *sdkrouter.Router, userID int) error {
	_, err := rt.Client(userID).WalletRemove(sdkrouter.WalletID(userID))
	if err != nil {
		return NewWalletError(userID, err)
	}
	Logger.LogF(monitor.F{"user_id": userID}).Info("wallet unloaded")
	return nil
}
