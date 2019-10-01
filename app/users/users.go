package users

import (
	"database/sql"

	"github.com/lbryio/lbrytv/internal/lbrynet"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/models"

	"github.com/lib/pq"
	"github.com/pkg/errors"
	xerrors "github.com/pkg/errors"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
)

// UserService stores manipulated user data
type UserService struct {
	logger monitor.ModuleLogger
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

// NewUserService returns UserService instance for retrieving or creating user records and accounts.
// Deprecated: NewWalletService should be used instead
func NewUserService() *UserService {
	s := &UserService{logger: monitor.NewModuleLogger("users")}
	return s
}

func (s *UserService) getDBUser(id int) (*models.User, error) {
	return models.Users(models.UserWhere.ID.EQ(id)).OneG()
}

func (s *UserService) createDBUser(id int) (*models.User, error) {
	log := s.logger.LogF(monitor.F{"id": id})

	u := &models.User{}
	u.ID = id
	err := u.InsertG(boil.Infer())

	if err != nil {
		// Check if we encountered a primary key violation, it would mean another routine
		// fired from another request has managed to create a user before us so we should try retrieving it again.
		switch baseErr := xerrors.Cause(err).(type) {
		case *pq.Error:
			if baseErr.Code == errUniqueViolation && baseErr.Column == "users_pkey" {
				log.Debug("user creation conflict, trying to retrieve local user again")
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

// Retrieve authenticates user with internal-api and retrieves/creates locally stored user.
// Deprecated: WalletService.Retrieve should be used instead
func (s *UserService) Retrieve(q Query) (*models.User, error) {
	token := q.Token
	log := s.logger.LogF(monitor.F{"token": token})
	var localUser *models.User

	remoteUser, err := getRemoteUser(token, q.MetaRemoteIP)
	if err != nil {
		log.Info("couldn't authenticate user with internal-apis")
		return nil, errors.Errorf("cannot authenticate user with internal-apis: %v", err)
	}

	log = s.logger.LogF(monitor.F{"token": token, "id": remoteUser.ID, "email": remoteUser.Email})

	if remoteUser.Email == "" {
		log.Info("cannot authenticate internal-api user: email not confirmed")
		return nil, errors.New("cannot authenticate user: email not confirmed")
	}

	localUser, errStorage := s.getDBUser(remoteUser.ID)
	if errStorage == sql.ErrNoRows {
		log.Infof("user ID=%v not found in the database, creating", remoteUser.ID)
		localUser, err = s.createDBUser(remoteUser.ID)
		if err != nil {
			return nil, err
		}

		err = s.createSDKAccount(localUser)
		if err != nil {
			return nil, err
		}
	} else if errStorage != nil {
		return nil, errStorage
	}

	if localUser.SDKAccountID.IsZero() {
		log.Warnf("user ID=%v has empty fields in the database, retrieving from the SDK", remoteUser.ID)
		acc, err := lbrynet.GetAccount(remoteUser.ID)
		if err != nil {
			monitor.CaptureException(err, map[string]string{"internal-api-id": string(remoteUser.ID)})
			log.Errorf("could not retrieve user ID=%v from the SDK: %v", remoteUser.ID, err)
			return nil, err
		}
		err = s.saveAccFields(savedAccFields{ID: acc.ID, PublicKey: acc.PublicKey}, localUser)
	}

	return localUser, nil
}

func (s *UserService) createSDKAccount(u *models.User) error {
	newAccount, err := lbrynet.CreateAccount(u.ID)
	if err != nil {
		switch err.(type) {
		case lbrynet.AccountConflict:
			s.logger.Log().Info("account creation conflict, proceeding")
		default:
			return err
		}
	} else {
		err = s.saveAccFields(savedAccFields{ID: newAccount.ID, PublicKey: newAccount.PublicKey}, u)
		if err != nil {
			return err
		}
		s.logger.Log().Info("created an sdk account")
	}
	return nil
}

func (s *UserService) saveAccFields(accFields savedAccFields, u *models.User) error {
	u.SDKAccountID = null.NewString(accFields.ID, true)
	_, err := u.UpdateG(boil.Infer())
	return err
}
