package db

import (
	"context"
	"errors"
	"net/http"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/lbrynet"
	"github.com/lbryio/lbrytv/models"
	"github.com/lbryio/lbrytv/monitor"

	"github.com/lbryio/lbry.go/extras/lbryinc"
	"github.com/volatiletech/sqlboiler/boil"
)

// TokenHeader is the name of HTTP header which is supplied by client and should contain internal-api auth_token
const TokenHeader string = "X-Lbry-Auth-Token"

// GetUserByToken retrieves user by internal-api auth_token
// TODO: Refactor out into logical pieces
// TODO: Implement different error types for better error messages
func GetUserByToken(token string) (*models.User, error) {
	var u *models.User
	ctx := context.Background()

	// This helps to ensure we don't try to look up and create users when multiple requests
	// with the same auth token header come in from the client - and the auth token isn't present in the database yet
	lockToken(token)
	defer releaseToken(token)

	u, err := models.Users(models.UserWhere.AuthToken.EQ(token)).OneG(ctx)
	if err != nil {
		Conn.Logger.LogF(monitor.F{monitor.TokenF: token}).Info("token not found in the database, trying internal-apis")
		c := lbryinc.NewClient(token)
		c.ServerAddress = config.Settings.GetString("InternalAPIHost")
		r, err := c.UserMe()
		if err != nil {
			Conn.Logger.LogF(monitor.F{monitor.TokenF: token}).Error("internal-api responded with an error")
			// No user found in internal-apis database, give up at this point
			return nil, err
		}

		email, verifiedEmail := r["primary_email"].(string)
		if !verifiedEmail {
			email = ""
			Conn.Logger.LogF(monitor.F{monitor.TokenF: token, "email": email}).Info("got an anonymous account from internal-apis")
		} else {
			Conn.Logger.LogF(monitor.F{monitor.TokenF: token, "email": email}).Info("got an account from internal-apis")
		}

		a, err := lbrynet.CreateAccount(email)
		if err != nil {
			return nil, err
		}

		u = new(models.User)
		u.Email = email
		u.AuthToken = token
		u.HasVerifiedEmail = verifiedEmail
		u.SDKAccountID = a.ID
		u.PrivateKey = a.PrivateKey
		u.PublicKey = a.PublicKey
		u.Seed = a.Seed
		err = u.InsertG(ctx, boil.Infer())

		if err != nil {
			Conn.Logger.LogF(monitor.F{monitor.TokenF: token, "email": email}).Error("error inserting a record, rolling back account", err)
			_, errDelete := lbrynet.RemoveAccount(email)
			if errDelete != nil {
				Conn.Logger.LogF(monitor.F{monitor.TokenF: token, "email": email}).Error("error rolling back account", errDelete)
			}
			return nil, err
		}
		Conn.Logger.LogF(monitor.F{monitor.TokenF: token, "email": email}).Info("saved a new account to the database")
	}
	return u, nil
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
