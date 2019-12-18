package users

import (
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/models"

	"github.com/lib/pq"
	xerrors "github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/boil"
)

// UserService stores manipulated user data
type UserService struct {
	Logger monitor.ModuleLogger
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
	s := &UserService{Logger: monitor.NewModuleLogger("users")}
	return s
}

func (s *UserService) getDBUser(id int) (*models.User, error) {
	return models.Users(models.UserWhere.ID.EQ(id)).OneG()
}

func (s *UserService) createDBUser(id int) (*models.User, error) {
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
