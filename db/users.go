package db

import (
	"context"

	"github.com/lbryio/lbrytv/lbrynet"
	"github.com/lbryio/lbrytv/models"
	"github.com/lbryio/lbrytv/monitor"

	"github.com/lbryio/lbry.go/extras/lbryinc"
	"github.com/volatiletech/sqlboiler/boil"
)

func GetUser(token string) (*models.User, error) {
	var u *models.User
	ctx := context.Background()
	Init()
	boil.SetDB(Conn.DB)
	u, err := models.Users(models.UserWhere.AuthToken.EQ(token)).OneG(ctx)
	if err != nil {
		Conn.Logger.LogF(monitor.F{token: token}).Infof("token supplied but isn't present in the database")
		c := lbryinc.NewClient(token)
		r, err := c.UserMe()
		if err != nil {
			// No user found in internal-apis database, give up at this point
			return u, err
		}
		if email, ok := r["primary_email"].(string); ok {
			// We got a valid email address from internal-apis
			u, err := models.Users(models.UserWhere.AuthToken.EQ(email)).One(ctx, Conn.DB)
			if err != nil {
				// No user with that email in our database, create one with the daemon instance and save to the database
				a, err := lbrynet.CreateAccount(email)
				if err != nil {
					return nil, err
				}

				u.Email = email
				u.AuthToken = token
				u.IsIdentityVerified = false
				u.HasVerifiedEmail = false
				u.SDKAccountID = a.ID
				u.PrivateKey = a.PrivateKey
				u.PublicKey = a.PublicKey
				u.Seed = a.Seed
				err := u.InsertG(ctx, boil.Infer())

				if err != nil {
					// TODO: Rollback SDK account here
					return nil, err
				}
			}
		}
	}
	return u, err
}
