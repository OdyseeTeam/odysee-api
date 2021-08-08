package wallet

import (
	"errors"
	"time"

	"github.com/lbryio/lbrytv/internal/metrics"

	"github.com/lbryio/lbry.go/v2/extras/lbryinc"

	"golang.org/x/oauth2"
)

// remoteUser encapsulates internal-apis user data
type remoteUser struct {
	ID               int  `json:"user_id"`
	HasVerifiedEmail bool `json:"has_verified_email"`
	Cached           bool
}

func getRemoteUserLegacy(url, token string, remoteIP string) (remoteUser, error) {
	op := metrics.StartOperation(opName, "get_remote_user")
	defer op.End()

	c := lbryinc.NewClient(token, &lbryinc.ClientOpts{
		ServerAddress: url,
		RemoteIP:      remoteIP,
	})

	start := time.Now()
	r, err := c.UserHasVerifiedEmail()
	duration := time.Now().Sub(start).Seconds()

	if err != nil {
		if errors.As(err, &lbryinc.APIError{}) {
			metrics.IAPIAuthFailedDurations.Observe(duration)
		} else {
			metrics.IAPIAuthErrorDurations.Observe(duration)
		}
		return remoteUser{}, err
	}

	metrics.IAPIAuthSuccessDurations.Observe(duration)

	ru := remoteUser{
		ID:               int(r["user_id"].(float64)),
		HasVerifiedEmail: r["has_verified_email"].(bool),
	}

	return ru, nil
}

func getRemoteUser(url string, token oauth2.TokenSource, remoteIP string) (remoteUser, error) {
	op := metrics.StartOperation(opName, "get_remote_user")
	defer op.End()

	c := lbryinc.NewOauthClient(token, &lbryinc.ClientOpts{
		ServerAddress: url,
		RemoteIP:      remoteIP,
	})

	start := time.Now()
	r, err := c.UserHasVerifiedEmail()
	duration := time.Now().Sub(start).Seconds()

	if err != nil {
		if errors.As(err, &lbryinc.APIError{}) {
			metrics.IAPIAuthFailedDurations.Observe(duration)
		} else {
			metrics.IAPIAuthErrorDurations.Observe(duration)
		}
		return remoteUser{}, err
	}

	metrics.IAPIAuthSuccessDurations.Observe(duration)

	ru := remoteUser{
		ID:               int(r["user_id"].(float64)),
		HasVerifiedEmail: r["has_verified_email"].(bool),
	}

	return ru, nil
}
