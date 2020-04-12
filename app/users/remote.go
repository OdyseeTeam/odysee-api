package users

import (
	"time"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/metrics"

	"github.com/lbryio/lbry.go/v2/extras/lbryinc"
)

// remoteUser encapsulates internal-apis user data
type remoteUser struct {
	ID               int
	HasVerifiedEmail bool
}

func getRemoteUser(token string, remoteIP string) (*remoteUser, error) {
	c := lbryinc.NewClient(token, &lbryinc.ClientOpts{
		ServerAddress: config.GetInternalAPIHost(),
		RemoteIP:      remoteIP,
	})

	start := time.Now()
	r, err := c.UserHasVerifiedEmail()
	duration := time.Now().Sub(start).Seconds()

	if err != nil {
		// No user found in internal-apis database, give up at this point
		metrics.IAPIAuthFailedDurations.Observe(duration)
		return nil, err
	}

	metrics.IAPIAuthSuccessDurations.Observe(duration)

	return &remoteUser{
		ID:               int(r["user_id"].(float64)),
		HasVerifiedEmail: r["has_verified_email"].(bool),
	}, nil
}
