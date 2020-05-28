package wallet

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/internal/errors"
	"github.com/lbryio/lbrytv/internal/lbrynet"
	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/internal/storage"
	"github.com/lbryio/lbrytv/models"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"

	"github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

var logger = monitor.NewModuleLogger("wallet")

func DisableLogger() { logger.Disable() } // for testing

// TokenHeader is the name of HTTP header which is supplied by client and should contain internal-api auth_token.
const TokenHeader = "X-Lbry-Auth-Token"
const pgUniqueConstraintViolation = "23505"

// GetUserWithWallet gets user by internal-apis auth token. If the user does not have a
// wallet yet, they are assigned an SDK and a wallet is created for them on that SDK.
func GetUserWithSDKServer(rt *sdkrouter.Router, internalAPIHost, token, metaRemoteIP string) (*models.User, error) {
	log := logger.WithFields(logrus.Fields{monitor.TokenF: token, "ip": metaRemoteIP})

	remoteUser, err := getRemoteUser(internalAPIHost, token, metaRemoteIP)
	if err != nil {
		msg := "authentication error: %v"
		log.Errorf(msg, err)
		return nil, fmt.Errorf(msg, err)
	}
	if !remoteUser.HasVerifiedEmail {
		return nil, nil
	}

	log.Data["remote_user_id"] = remoteUser.ID
	log.Data["has_email"] = remoteUser.HasVerifiedEmail
	log.Debugf("user authenticated")

	ctx, cancelFn := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelFn()

	var localUser *models.User
	err = tx(ctx, storage.Conn.DB.DB, func(tx *sql.Tx) error {
		localUser, err = getOrCreateLocalUser(tx, remoteUser.ID, log)
		if err != nil {
			return err
		}

		if localUser.LbrynetServerID.IsZero() {
			err := assignSDKServerToUser(tx, localUser, rt.LeastLoaded(), log)
			if err != nil {
				return err
			}
		}
		return nil
	})

	return localUser, err
}

func tx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	err = fn(tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func getOrCreateLocalUser(exec boil.Executor, remoteUserID int, log *logrus.Entry) (*models.User, error) {
	localUser, err := getDBUser(exec, remoteUserID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	} else if err == sql.ErrNoRows {
		log.Infof("user not found in the database, creating")
		localUser, err = createDBUser(exec, remoteUserID)
		if err != nil {
			return nil, err
		}
	} else if localUser.LbrynetServerID.IsZero() {
		// Should not happen, but not enforced in DB structure yet
		log.Errorf("user %d found in db but doesn't have sdk assigned", localUser.ID)
	}

	return localUser, nil
}

func createDBUser(exec boil.Executor, id int) (*models.User, error) {
	log := logger.WithFields(logrus.Fields{"id": id})

	u := &models.User{ID: id}
	err := u.Insert(exec, boil.Infer())
	if err == nil {
		metrics.LbrytvNewUsers.Inc()
		return u, nil
	}

	// Check if we encountered a primary key violation, it would mean another routine
	// fired from another request has managed to create a user before us so we should try retrieving it again.
	var pgErr *pq.Error
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueConstraintViolation {
		log.Info("user creation conflict, trying to retrieve the local user again")
		return getDBUser(exec, id)
	}
	log.Error("unknown error encountered while creating user:", err)
	return nil, err
}

func getDBUser(exec boil.Executor, id int) (*models.User, error) {
	return models.Users(
		models.UserWhere.ID.EQ(id),
		qm.Load(models.UserRels.LbrynetServer),
	).One(exec)
}

// assignSDKServerToUser permanently assigns an sdk to a user, and creates a wallet on that sdk for that user.
// it ensures that the assigned sdk is set on user.R.LbrynetServer, so it can be accessed externally
func assignSDKServerToUser(exec boil.Executor, user *models.User, server *models.LbrynetServer, log *logrus.Entry) error {
	if user.ID == 0 {
		return errors.Err("user must already exist in db")
	}
	if !user.LbrynetServerID.IsZero() {
		return errors.Err("user already has an sdk assigned")
	}

	if server.ID == 0 {
		// THIS SERVER CAME FROM A CONFIG FILE, NOT THE DB (prolly during testing)
		// TODO: handle this case better
		log.Warnf("user %d is getting an sdk with no ID. could happen if servers came from config file", user.ID)
		return Create(server.Address, user.ID)
	}

	log.Debugf("user %d: trying to assign sdk %s (%s)", user.ID, server.Name, server.Address)
	needsWalletCreation := true

	// atomic update. it checks that lbrynet_server_id is null before updating
	q := fmt.Sprintf(`UPDATE "%s" SET "%s" = $1 WHERE "%s" = $2 and "%s" IS NULL`,
		models.TableNames.Users,
		models.UserColumns.LbrynetServerID,
		models.UserColumns.ID,
		models.UserColumns.LbrynetServerID,
	)
	result, err := exec.Exec(q, server.ID, user.ID)
	if err != nil {
		return errors.Err(err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return errors.Err(err)
	}
	if count == 0 {
		// update from another request got there first. reload user to get the assigned server
		err = user.ReloadG()
		if err != nil {
			return errors.Err(err)
		}
		needsWalletCreation = false // it will have been created by the request that did the successful assignment
		log.Debugf("user %d: already assigned to a server", user.ID)
		// TODO: sleep some time to give the other request time to actually create a wallet?
		// TODO: or keep a global "wallet creation in progress" locking/waiting setup?
	} else {
		user.LbrynetServerID.SetValid(server.ID)
	}

	// reload LbrynetServer relation
	if user.R == nil {
		user.R = user.R.NewStruct()
	}
	srv, err := user.LbrynetServer().One(exec)
	if err != nil {
		return errors.Err(err)
	}
	user.R.LbrynetServer = srv
	log.Infof("user %d: assigned to sdk %s (%s)", user.ID, server.Name, server.Address)

	if needsWalletCreation {
		return Create(server.Address, user.ID)
	}

	return nil
}

// Create creates a wallet on an sdk that can be immediately used in subsequent commands.
// It can recover from errors like existing wallets, but if a wallet is known to exist
// (eg. a wallet ID stored in the database already), loadWallet() should be called instead.
func Create(serverAddress string, userID int) error {
	err := createWallet(serverAddress, userID)
	if err == nil {
		return nil
	}

	log := logger.WithFields(logrus.Fields{"user_id": userID, "sdk": serverAddress})

	if errors.Is(err, lbrynet.ErrWalletExists) {
		log.Warn(err.Error())
		return nil
	}

	if errors.Is(err, lbrynet.ErrWalletNeedsLoading) {
		log.Info(err.Error())
		err = LoadWallet(serverAddress, userID)
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
	logger.WithFields(logrus.Fields{"user_id": userID, "sdk": addr}).Info("wallet created")
	return nil
}

// loadWallet loads an existing wallet in the LbrynetServer.
// May return errors:
//  WalletAlreadyLoaded - wallet is already loaded and operational
//  WalletNotFound - wallet file does not exist and won't be loaded.
func LoadWallet(addr string, userID int) error {
	_, err := ljsonrpc.NewClient(addr).WalletAdd(sdkrouter.WalletID(userID))
	if err != nil {
		return lbrynet.NewWalletError(userID, err)
	}
	logger.WithFields(logrus.Fields{"user_id": userID, "sdk": addr}).Info("wallet loaded")
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
	logger.WithFields(logrus.Fields{"user_id": userID, "sdk": addr}).Info("wallet unloaded")
	return nil
}
