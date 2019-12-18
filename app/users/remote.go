package users

import (
	"time"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/metrics"

	"github.com/lbryio/lbry.go/v2/extras/lbryinc"
)

// RemoteUser encapsulates internal-apis user data
type RemoteUser struct {
	ID               int
	HasVerifiedEmail bool
}

func getRemoteUser(token string, remoteIP string) (*RemoteUser, error) {
	u := &RemoteUser{}
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

	u.ID = int(r["user_id"].(float64))
	u.HasVerifiedEmail = r["has_verified_email"].(bool)
	return u, nil
}
