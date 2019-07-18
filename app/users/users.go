package users

import (
	"database/sql"
	"net/http"

	"github.com/lbryio/lbrytv/internal/lbrynet"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/models"

	ljsonrpc "github.com/lbryio/lbry.go/extras/jsonrpc"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/boil"
)

// UserService stores manipulated user data
type UserService struct {
	token string
	log   *logrus.Entry
}

// TokenHeader is the name of HTTP header which is supplied by client and should contain internal-api auth_token.
const TokenHeader string = "X-Lbry-Auth-Token"
const idPrefix string = "id:"
const errUniqueViolation = "23505"

// NewUserService returns UserService instance for retrieving or creating user records and accounts.
func NewUserService(token string) *UserService {
	s := &UserService{
		token: token,
	}
	s.updateLogger(monitor.F{})
	return s
}

func (s *UserService) updateLogger(fields monitor.F) {
	fields[monitor.TokenF] = s.token
	s.log = monitor.NewModuleLogger("users").LogF(fields)
}

func (s *UserService) getLocalUser(id int) (*models.User, error) {
	return models.Users(models.UserWhere.ID.EQ(id)).OneG()
}

func (s *UserService) createLocalUser(id int) (*models.User, error) {
	u := new(models.User)
	u.ID = id
	err := u.InsertG(boil.Infer())

	if err != nil {
		// Check if we encountered a primary key violation, it would mean another routine
		// fired from another request has managed to create a user before us so we should try retrieving it again.
		switch baseErr := errors.Cause(err).(type) {
		case *pq.Error:
			if baseErr.Code == errUniqueViolation && baseErr.Column == "users_pkey" {
				s.log.Debug("user creation conflict, trying to retrieve local user again")
				u, retryErr := s.getLocalUser(id)
				if retryErr != nil {
					return nil, retryErr
				}
				return u, nil
			}
		default:
			s.log.Error("unknown error encountered while creating user: ", err)
			return nil, err
		}
	}
	return u, nil
}

// GetUser authenticates user with internal-api and retrieves/creates locally stored user.
func (s *UserService) GetUser() (*models.User, error) {
	var localUser *models.User

	remoteUser, err := getRemoteUser(s.token)
	if err != nil {
		s.log.Info("couldn't authenticate user with internal-apis")
		return nil, errors.Errorf("cannot authenticate user with internal-apis: %v", err)
	}

	s.updateLogger(monitor.F{"id": remoteUser.ID, "email": remoteUser.Email})

	if remoteUser.Email == "" {
		s.log.Info("empty email for internal-api user")
		return nil, errors.New("cannot authenticate user: email is empty/not confirmed")
	}

	localUser, errStorage := s.getLocalUser(remoteUser.ID)
	if errStorage == sql.ErrNoRows {
		localUser, err = s.createLocalUser(remoteUser.ID)
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

	return localUser, nil
}

func (s *UserService) createSDKAccount(u *models.User) error {
	newAccount, err := lbrynet.CreateAccount(u.ID)
	if err != nil {
		switch err.(type) {
		case lbrynet.AccountConflict:
			s.log.Info("account creation conflict, proceeding")
		default:
			return err
		}
	} else {
		err = s.saveSDKFields(newAccount, u)
		if err != nil {
			return err
		}
		s.log.Info("created an sdk account")
	}
	return nil
}

func (s *UserService) saveSDKFields(a *ljsonrpc.AccountCreateResponse, u *models.User) error {
	u.SDKAccountID = a.ID
	u.PrivateKey = a.PrivateKey
	u.PublicKey = a.PublicKey
	u.Seed = a.Seed
	_, err := u.UpdateG(boil.Infer())
	return err
}

// GetAccountIDFromRequest retrieves SDK  account_id of a user making a http request
// by a header provided by http client.
func GetAccountIDFromRequest(req *http.Request) (string, error) {
	if token, ok := req.Header[TokenHeader]; ok {
		s := NewUserService(token[0])
		u, err := s.GetUser()
		if err != nil {
			return "", err
		}
		if u == nil {
			return "", errors.New("unable to retrieve user")
		}
		return u.SDKAccountID, nil
	}
	return "", nil
}
