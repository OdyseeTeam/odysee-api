package users

import (
	"database/sql"
	"fmt"

	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/models"

	"github.com/lib/pq"
	xerrors "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/boil"
)

// WalletService retrieves user wallet data.
type WalletService struct {
	Logger monitor.ModuleLogger
	Router *sdkrouter.Router
}

// TokenHeader is the name of HTTP header which is supplied by client and should contain internal-api auth_token.
const TokenHeader string = "X-Lbry-Auth-Token"
const errUniqueViolation = "23505"

// Retriever is an interface for user retrieval by internal-apis auth token
type Retriever interface {
	Retrieve(query Query) (*models.User, error)
}

// Query contains queried user details and optional metadata about the request
type Query struct {
	Token        string
	MetaRemoteIP string
}

// NewWalletService returns WalletService instance for retrieving or creating wallet-based user records and accounts.
func NewWalletService(r *sdkrouter.Router) *WalletService {
	return &WalletService{Logger: monitor.NewModuleLogger("users"), Router: r}
}

func (s *WalletService) createDBUser(id int) (*models.User, error) {
	log := s.Logger.LogF(monitor.F{"id": id})

	u := &models.User{ID: id}
	err := u.InsertG(boil.Infer())
	if err != nil {
		// Check if we encountered a primary key violation, it would mean another routine
		// fired from another request has managed to create a user before us so we should try retrieving it again.
		switch baseErr := xerrors.Cause(err).(type) {
		case *pq.Error:
			if baseErr.Code == errUniqueViolation && baseErr.Column == "users_pkey" {
				log.Debug("user creation conflict, trying to retrieve the local user again")
				return getDBUser(id)
			}
		default:
			log.Error("unknown error encountered while creating user: ", err)
			return nil, err
		}
	}
	return u, nil
}

// Retrieve gets user by internal-apis auth token provided in the supplied Query.
func (s *WalletService) Retrieve(q Query) (*models.User, error) {
	token := q.Token
	log := s.Logger.LogF(monitor.F{monitor.TokenF: token})

	remoteUser, err := getRemoteUser(token, q.MetaRemoteIP)
	if err != nil {
		msg := "cannot authenticate user with internal-apis: %v"
		log.Errorf(msg, err)
		return nil, fmt.Errorf(msg, err)
	}
	if !remoteUser.HasVerifiedEmail {
		return nil, nil
	}

	log.Data["id"] = remoteUser.ID
	log.Data["has_email"] = remoteUser.HasVerifiedEmail

	localUser, err := getDBUser(remoteUser.ID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	} else if err == sql.ErrNoRows {
		log.Infof("user not found in the database, creating")
		localUser, err = s.createDBUser(remoteUser.ID)
		if err != nil {
			return nil, err
		}
	} else if localUser.WalletID == "" {
		// This scenario may happen for legacy users who are present in the database but don't have a wallet yet
		log.Warnf("user %d doesn't have wallet ID set", localUser.ID)
	}

	if localUser.WalletID == "" {
		err := createWalletForUser(localUser, s.Router, log)
		if err != nil {
			return nil, err
		}
	}

	return localUser, nil
}

func createWalletForUser(user *models.User, router *sdkrouter.Router, log *logrus.Entry) error {
	// either a new user or a legacy user without a wallet
	walletID, err := router.InitializeWallet(user.ID)
	if err != nil {
		return err
	}

	log.Data["wallet_id"] = walletID
	log.Info("saving wallet ID to user record")

	user.WalletID = walletID

	server := router.GetServer(user.ID)
	if server.ID > 0 { // Ensure server is from DB
		user.LbrynetServerID.SetValid(server.ID)
	}

	_, err = user.UpdateG(boil.Infer())
	return err
}

func getDBUser(id int) (*models.User, error) {
	return models.Users(models.UserWhere.ID.EQ(id)).OneG()
}
