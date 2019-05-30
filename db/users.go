package db

import (
	"context"
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

	u, err := models.Users(models.UserWhere.AuthToken.EQ(token)).OneG(ctx)
	if err != nil {
		Conn.Logger.LogF(monitor.F{"token": token}).Info("token supplied but isn't present in the database, trying internal-apis")
		c := lbryinc.NewClient(token)
		c.ServerAddress = config.Settings.GetString("InternalAPIHost")
		r, err := c.UserMe()
		if err != nil {
			Conn.Logger.LogF(monitor.F{"token": token}).Error("internal-api responded with an error")
			// No user found in internal-apis database, give up at this point
			return u, err
		}
		if email, ok := r["primary_email"].(string); ok {
			Conn.Logger.LogF(monitor.F{"token": token, "email": email}).Info("got an account from internal-apis")
			u, err := models.Users(models.UserWhere.AuthToken.EQ(email)).One(ctx, Conn.DB)
			if err != nil {
				Conn.Logger.LogF(monitor.F{"token": token, "email": email}).Info("user seen first time, creating locally")
				a, err := lbrynet.CreateAccount(email)
				if err != nil {
					return nil, err
				}

				u = new(models.User)
				u.Email = email
				u.AuthToken = token
				u.IsIdentityVerified = false
				u.HasVerifiedEmail = false
				u.SDKAccountID = a.ID
				u.PrivateKey = a.PrivateKey
				u.PublicKey = a.PublicKey
				u.Seed = a.Seed
				err = u.InsertG(ctx, boil.Infer())

				if err != nil {
					Conn.Logger.LogF(monitor.F{"token": token, "email": email}).Error("error inserting a record, rolling back account")
					_, errDelete := lbrynet.RemoveAccount(email)
					if errDelete != nil {
						Conn.Logger.LogF(monitor.F{"token": token, "email": email}).Errorf("error rolling back account: %v", errDelete)
					}
					return nil, err
				}
				return u, nil
			}
		}
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
		return u.SDKAccountID, nil
	}
	return "", nil
}
