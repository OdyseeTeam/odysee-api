package wallet

import (
	"database/sql"
	"errors"
	"fmt"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/internal/lbrynet"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/models"
	"github.com/volatiletech/sqlboiler/queries/qm"

	"github.com/lib/pq"
	pkgerrors "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/boil"
)

var logger = monitor.NewModuleLogger("wallet")

func DisableLogger() { logger.Disable() } // for testing

// TokenHeader is the name of HTTP header which is supplied by client and should contain internal-api auth_token.
const TokenHeader = "X-Lbry-Auth-Token"
const pgUniqueConstraintViolation = "23505"

// Retrieve gets user by internal-apis auth token. If the user does not have a wallet yet, they
// are assigned an SDK and a wallet is created for them on that SDK.
func GetUserWithWallet(rt *sdkrouter.Router, internalAPIHost, token, metaRemoteIP string) (*models.User, error) {
	log := logger.LogF(monitor.F{monitor.TokenF: token})

	remoteUser, err := getRemoteUser(internalAPIHost, token, metaRemoteIP)
	if err != nil {
		msg := "cannot authenticate user with internal-apis: %v"
		log.Errorf(msg, err)
		return nil, fmt.Errorf(msg, err)
	}
	if !remoteUser.HasVerifiedEmail {
		return nil, nil
	}

	log.Data["remote_user_id"] = remoteUser.ID
	log.Data["has_email"] = remoteUser.HasVerifiedEmail

	localUser, err := getOrCreateLocalUser(remoteUser.ID, log)
	if err != nil {
		return nil, err
	}

	if localUser.LbrynetServerID.IsZero() {
		err := assignSDKServerToUser(localUser, rt, log)
		if err != nil {
			return nil, err
		}
	}

	return localUser, nil
}

func getOrCreateLocalUser(remoteUserID int, log *logrus.Entry) (*models.User, error) {
	localUser, err := getDBUser(remoteUserID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	} else if err == sql.ErrNoRows {
		log.Infof("user not found in the database, creating")
		localUser, err = createDBUser(remoteUserID)
		if err != nil {
			return nil, err
		}
	} else if localUser.LbrynetServerID.IsZero() {
		// This scenario may happen for legacy users who are present in the database but don't have a server assigned
		log.Warnf("user %d found in db but doesn't have sdk assigned", localUser.ID)
	}

	return localUser, nil
}

func assignSDKServerToUser(user *models.User, router *sdkrouter.Router, log *logrus.Entry) error {
	server := router.LeastLoaded()
	if server.ID > 0 { // Ensure server is from DB
		user.LbrynetServerID.SetValid(server.ID)
	} else {
		// THIS SERVER CAME FROM A CONFIG FILE (prolly during testing)
		// TODO: handle this case better
		log.Warnf("user %d is getting an sdk with no ID. could happen if servers came from config file", user.ID)
	}

	err := Create(server.Address, user.ID)
	if err != nil {
		return err
	}

	log.Infof("assigning sdk %s to user %d", server.Address, user.ID)
	_, err = user.UpdateG(boil.Infer())
	return err
}

func createDBUser(id int) (*models.User, error) {
	log := logger.LogF(monitor.F{"id": id})

	u := &models.User{ID: id}
	err := u.InsertG(boil.Infer())
	if err == nil {
		return u, nil
	}

	// Check if we encountered a primary key violation, it would mean another routine
	// fired from another request has managed to create a user before us so we should try retrieving it again.
	switch baseErr := pkgerrors.Cause(err).(type) {
	case *pq.Error:
		if baseErr.Code == pgUniqueConstraintViolation && baseErr.Column == "users_pkey" {
			log.Debug("user creation conflict, trying to retrieve the local user again")
			return getDBUser(id)
		}
	}

	log.Error("unknown error encountered while creating user: ", err)
	return nil, err
}

func getDBUser(id int) (*models.User, error) {
	return models.Users(
		models.UserWhere.ID.EQ(id),
		qm.Load(models.UserRels.LbrynetServer),
	).OneG()
}

// Create creates a wallet on an sdk that can be immediately used in subsequent commands.
// It can recover from errors like existing wallets, but if a wallet is known to exist
// (eg. a wallet ID stored in the database already), loadWallet() should be called instead.
func Create(serverAddress string, userID int) error {
	err := createWallet(serverAddress, userID)
	if err == nil {
		return nil
	}

	log := logger.LogF(monitor.F{"user_id": userID, "sdk": serverAddress})

	if errors.Is(err, lbrynet.ErrWalletExists) {
		log.Warn(err.Error())
		return nil
	}

	if errors.Is(err, lbrynet.ErrWalletNeedsLoading) {
		log.Info(err.Error())
		err = loadWallet(serverAddress, userID)
		if err != nil {
			if errors.Is(err, lbrynet.ErrWalletAlreadyLoaded) {
				log.Info(err.Error())
				return nil
			}
			return err
		}
		return nil
	}

	log.Errorf("don't know how to recover from error: %v", err)
	return err
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
func createWallet(addr string, userID int) error {
	_, err := ljsonrpc.NewClient(addr).WalletCreate(sdkrouter.WalletID(userID), &ljsonrpc.WalletCreateOpts{
		SkipOnStartup: true, CreateAccount: true, SingleKey: true})
	if err != nil {
		return lbrynet.NewWalletError(userID, err)
	}
	logger.LogF(monitor.F{"user_id": userID, "sdk": addr}).Info("wallet created")
	return nil
}

// loadWallet loads an existing wallet in the LbrynetServer.
// May return errors:
//  WalletAlreadyLoaded - wallet is already loaded and operational
//  WalletNotFound - wallet file does not exist and won't be loaded.
func loadWallet(addr string, userID int) error {
	_, err := ljsonrpc.NewClient(addr).WalletAdd(sdkrouter.WalletID(userID))
	if err != nil {
		return lbrynet.NewWalletError(userID, err)
	}
	logger.LogF(monitor.F{"user_id": userID, "sdk": addr}).Info("wallet loaded")
	return nil
}

// UnloadWallet unloads an existing wallet from the LbrynetServer.
// May return errors:
//  WalletAlreadyLoaded - wallet is already loaded and operational
//  WalletNotFound - wallet file does not exist and won't be loaded.
func UnloadWallet(addr string, userID int) error {
	_, err := ljsonrpc.NewClient(addr).WalletRemove(sdkrouter.WalletID(userID))
	if err != nil {
		return lbrynet.NewWalletError(userID, err)
	}
	logger.LogF(monitor.F{"user_id": userID, "sdk": addr}).Info("wallet unloaded")
	return nil
}
