package users

import (
	"database/sql"
	"fmt"

	"github.com/lbryio/lbrytv/app/router"
	"github.com/lbryio/lbrytv/internal/lbrynet"
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
	Router *router.SDK
}

// TokenHeader is the name of HTTP header which is supplied by client and should contain internal-api auth_token.
const TokenHeader string = "X-Lbry-Auth-Token"
const idPrefix string = "id:"
const errUniqueViolation = "23505"

type savedAccFields struct {
	ID        string
	PublicKey string
}

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
func NewWalletService(r *router.SDK) *WalletService {
	s := &WalletService{Logger: monitor.NewModuleLogger("users"), Router: r}
	return s
}

func (s *WalletService) getDBUser(id int) (*models.User, error) {
	return models.Users(models.UserWhere.ID.EQ(id)).OneG()
}

func (s *WalletService) createDBUser(id int) (*models.User, error) {
	log := s.Logger.LogF(monitor.F{"id": id})

	u := &models.User{}
	u.ID = id
	err := u.InsertG(boil.Infer())

	if err != nil {
		// Check if we encountered a primary key violation, it would mean another routine
		// fired from another request has managed to create a user before us so we should try retrieving it again.
		switch baseErr := xerrors.Cause(err).(type) {
		case *pq.Error:
			if baseErr.Code == errUniqueViolation && baseErr.Column == "users_pkey" {
				log.Debug("user creation conflict, trying to retrieve the local user again")
				u, retryErr := s.getDBUser(id)
				if retryErr != nil {
					return nil, retryErr
				}
				return u, nil
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
	var (
		localUser     *models.User
		lbrynetServer *models.LbrynetServer
		wid           string
	)

	token := q.Token

	log := s.Logger.LogF(monitor.F{monitor.TokenF: token})

	remoteUser, err := getRemoteUser(token, q.MetaRemoteIP)
	if err != nil {
		return nil, s.LogErrorAndReturn(log, "cannot authenticate user with internal-apis: %v", err)
	}

	// Update log entry with extra context data
	log = s.Logger.LogF(monitor.F{
		monitor.TokenF: token,
		"id":           remoteUser.ID,
		"has_email":    remoteUser.HasVerifiedEmail,
	})
	if !remoteUser.HasVerifiedEmail {
		return nil, nil
	}

	localUser, errStorage := s.getDBUser(remoteUser.ID)
	if errStorage == sql.ErrNoRows {
		log.Infof("user not found in the database, creating")
		localUser, err = s.createDBUser(remoteUser.ID)
		if err != nil {
			return nil, err
		}

		lbrynetServer, wid, err = s.createWallet(localUser)
		if err != nil {
			return nil, err
		}

		err := s.postCreateUpdate(localUser, lbrynetServer, wid)
		if err != nil {
			return nil, err
		}

		log.Data["wallet_id"] = wid
	} else if errStorage != nil {
		return nil, errStorage
	}

	// This scenario may happen for legacy users who are present in the database but don't have a wallet yet
	if localUser.WalletID == "" {
		log.Warn("user doesn't have wallet ID set")
		lbrynetServer, wid, err = s.createWallet(localUser)
		if err != nil {
			return nil, err
		}

		err := s.postCreateUpdate(localUser, lbrynetServer, wid)
		if err != nil {
			return nil, err
		}
	}

	return localUser, nil
}

func (s *WalletService) createWallet(u *models.User) (*models.LbrynetServer, string, error) {
	return lbrynet.InitializeWallet(s.Router, u.ID)
}

func (s *WalletService) postCreateUpdate(u *models.User, server *models.LbrynetServer, wid string) error {
	s.Logger.LogF(monitor.F{"id": u.ID, "wallet_id": wid}).Info("saving wallet ID to user record")
	u.WalletID = wid
	if server.ID > 0 { //Ensure server is from DB
		u.LbrynetServerID.SetValid(server.ID)
	}

	_, err := u.UpdateG(boil.Infer())
	return err
}

// LogErrorAndReturn logs error with rich context and returns an error object
// so it can be returned from the function
func (s *WalletService) LogErrorAndReturn(log *logrus.Entry, message string, a ...interface{}) error {
	log.Errorf(message, a...)
	return fmt.Errorf(message, a...)
}
