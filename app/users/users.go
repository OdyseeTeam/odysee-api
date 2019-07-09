package users

import (
	"database/sql"
	"net/http"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/lbrynet"
	"github.com/lbryio/lbrytv/models"
	"github.com/lbryio/lbrytv/internal/monitor"

	ljsonrpc "github.com/lbryio/lbry.go/extras/jsonrpc"
	"github.com/lbryio/lbry.go/extras/lbryinc"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/boil"
)

type UserService struct {
	logger  monitor.ModuleLogger
}

// TokenHeader is the name of HTTP header which is supplied by client and should contain internal-api auth_token
const TokenHeader string = "X-Lbry-Auth-Token"
const idPrefix string = "id:"
const errUniqueViolation = "23505"

func getRemoteUser(token string) map[string]interface{} {
	c := lbryinc.NewClient(token)
	c.ServerAddress = config.GetInternalAPIHost()
	r, err := c.UserMe()
	if err != nil {
		// Conn.Logger.LogF(monitor.F{monitor.TokenF: token}).Error("internal-api responded with an error: ", err)
		// No user found in internal-apis database, give up at this point
		return nil
	}
	return r
}

// GetUserByToken retrieves user by internal-api auth_token
func GetUserByToken(token string) (*models.User, error) {
	var (
		u     *models.User
		id    int
		email string
	)

	rUser := getRemoteUser(token)
	if rUser == nil {
		// Conn.Logger.LogF(monitor.F{monitor.TokenF: token}).Info("couldn't authenticate user with internal-apis")
		return nil, errors.New("cannot authenticate user with internal-apis")
	}
	id = int(rUser["id"].(float64))

	if rUser["primary_email"] != nil {
		email = rUser["primary_email"].(string)
	}

	logger := monitor.NewModuleLogger("users").LogF(monitor.F{monitor.TokenF: token, "id": id, "email": email})

	u, err := getLocalUser(id)

	if err == sql.ErrNoRows {
		u = new(models.User)
		u.ID = id
		err = u.InsertG(boil.Infer())

		if err != nil {
			// Check if we encountered a primary key violation, it would mean another goroutine
			// has created a user before us so we should try retrieving it again.
			switch baseErr := errors.Cause(err).(type) {
			case *pq.Error:
				if baseErr.Code == errUniqueViolation && baseErr.Column == "users_pkey" {
					logger.Debug("user creation conflict, trying to retrieve local user again")
					u, retryErr := getLocalUser(id)
					if retryErr != nil {
						return u, retryErr
					}
				}
			default:
				logger.Error("unknown error encountered while creating user: ", err)
				return nil, err
			}
		}

		newAccount, sdkErr := lbrynet.CreateAccount(id)
		if sdkErr != nil {
			switch sdkErr.(type) {
			case lbrynet.AccountConflict:
				logger.Info("account creation conflict, proceeding")
			default:
				return u, err
			}
		} else {
			err = renderAccountOntoLocalUser(newAccount, u)
			if err != nil {
				return nil, err
			}
			logger.Info("created an sdk account")
		}
	} else if err != nil {
		return u, err
	}
	return u, nil
}

func getLocalUser(id int) (*models.User, error) {
	return models.Users(models.UserWhere.ID.EQ(id)).OneG()
}

func renderAccountOntoLocalUser(a *ljsonrpc.AccountCreateResponse, u *models.User) error {
	u.SDKAccountID = a.ID
	u.PrivateKey = a.PrivateKey
	u.PublicKey = a.PublicKey
	u.Seed = a.Seed
	_, err := u.UpdateG(boil.Infer())
	return err
}

// GetAccountIDFromRequest retrieves SDK  account_id of a user making a http request
// by a header provided by the http client
func GetAccountIDFromRequest(req *http.Request) (string, error) {
	if token, ok := req.Header[TokenHeader]; ok {
		u, err := GetUserByToken(token[0])
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
