package wallet

import (
	"time"

	"github.com/lbryio/lbrytv/internal/metrics"

	"github.com/lbryio/lbry.go/v2/extras/lbryinc"
)

// remoteUser encapsulates internal-apis user data
type remoteUser struct {
	ID               int  `json:"user_id"`
	HasVerifiedEmail bool `json:"has_verified_email"`
	Cached           bool
}

func getRemoteUser(url, token string, remoteIP string) (remoteUser, error) {
	if uid := currentCache.get(token); uid != 0 {
		return remoteUser{ID: uid, HasVerifiedEmail: true, Cached: true}, nil
	}

	c := lbryinc.NewClient(token, &lbryinc.ClientOpts{
		ServerAddress: url,
		RemoteIP:      remoteIP,
	})

	start := time.Now()
	r, err := c.UserHasVerifiedEmail()
	duration := time.Now().Sub(start).Seconds()

	if err != nil {
		// No user found in internal-apis database, give up at this point
		metrics.IAPIAuthFailedDurations.Observe(duration)
		return remoteUser{}, err
	}

	metrics.IAPIAuthSuccessDurations.Observe(duration)

	ru := remoteUser{
		ID:               int(r["user_id"].(float64)),
		HasVerifiedEmail: r["has_verified_email"].(bool),
	}
	currentCache.set(token, ru.ID)

	return ru, nil
}
