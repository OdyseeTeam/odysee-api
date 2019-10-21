package users

import (
	"time"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/metrics"

	"github.com/lbryio/lbry.go/v2/extras/lbryinc"
)

// RemoteUser encapsulates internal-apis user data
type RemoteUser struct {
	ID    int
	Email string
}

func getRemoteUser(token string, remoteIP string) (*RemoteUser, error) {
	u := &RemoteUser{}
	c := lbryinc.NewClient(token, &lbryinc.ClientOpts{
		ServerAddress: config.GetInternalAPIHost(),
		RemoteIP:      remoteIP,
	})

	start := time.Now()
	r, err := c.UserMe()
	duration := time.Now().Sub(start).Seconds()

	if err != nil {
		// No user found in internal-apis database, give up at this point
		metrics.IAPIAuthFailedDurations.Observe(duration)
		return nil, err
	}
	metrics.IAPIAuthSuccessDurations.Observe(duration)

	u.ID = int(r["id"].(float64))
	if r["primary_email"] != nil {
		u.Email = r["primary_email"].(string)
	}
	return u, nil
}
