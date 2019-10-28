package users

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/lbryio/lbrytv/internal/lbrynet"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/models"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/boil"
)

type WalletService struct {
	UserService
}

// NewWalletService returns UserService instance for retrieving or creating wallet-based user records and accounts.
func NewWalletService() *WalletService {
	s := &WalletService{UserService{Logger: monitor.NewModuleLogger("users")}}
	return s
}

func (s *WalletService) Retrieve(q Query) (*models.User, error) {
	var (
		localUser *models.User
		wid       string
	)
	doWalletInit := true
	token := q.Token

	log := s.Logger.LogF(monitor.F{"token": token})

	remoteUser, err := getRemoteUser(token, q.MetaRemoteIP)
	if err != nil {
		return nil, s.LogErrorAndReturn(log, "cannot authenticate user with internal-apis: %v", err)
	}

	// Update log entry with extra context data
	log = s.Logger.LogF(monitor.F{"token": token, "id": remoteUser.ID, "email": remoteUser.Email})
	if remoteUser.Email == "" {
		return nil, nil
	}

	localUser, errStorage := s.getDBUser(remoteUser.ID)
	if errStorage == sql.ErrNoRows {
		log.Infof("user was not found in the database, creating")
		localUser, err = s.createDBUser(remoteUser.ID)
		if err != nil {
			return nil, err
		}

		wid, err = s.createWallet(localUser)
		if err != nil {
			return nil, err
		}

		err := s.saveWalletID(localUser, wid)
		if err != nil {
			return nil, err
		}

		doWalletInit = false
		log = s.Logger.LogF(monitor.F{"token": token, "id": remoteUser.ID, "email": remoteUser.Email, "wallet_id": wid})
	} else if errStorage != nil {
		return nil, errStorage
	}

	// This scenario may happen for legacy users who are present in the database but don't have a wallet yet
	if localUser.WalletID == "" {
		log.Warn("user doesn't have wallet ID set")
		wid, err = s.createWallet(localUser)
		if err != nil {
			return nil, err
		}
		doWalletInit = false
		err := s.saveWalletID(localUser, wid)
		if err != nil {
			return nil, err
		}
	}

	if doWalletInit {
		err = s.initializeWallet(localUser)
		if err != nil && !errors.As(err, &lbrynet.WalletAlreadyLoaded{}) {
			return nil, err
		}
	}
	return localUser, nil
}

func (s *WalletService) createWallet(u *models.User) (string, error) {
	return lbrynet.InitializeWallet(u.ID)
}

func (s *WalletService) initializeWallet(u *models.User) error {
	_, err := lbrynet.AddWallet(u.ID)
	return err
}

func (s *WalletService) saveWalletID(u *models.User, wid string) error {
	s.Logger.LogF(monitor.F{"id": u.ID, "wallet_id": wid}).Info("saving wallet ID to user record")
	u.WalletID = wid
	_, err := u.UpdateG(boil.Infer())
	return err
}

// LogErrorAndReturn logs error with rich context and returns an error object
// so it can be returned from the function
func (s *WalletService) LogErrorAndReturn(log *logrus.Entry, message string, a ...interface{}) error {
	log.Errorf(message, a...)
	return fmt.Errorf(message, a...)
}
