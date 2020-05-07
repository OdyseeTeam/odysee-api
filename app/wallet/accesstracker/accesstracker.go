package accesstracker

import (
	"fmt"
	"time"

	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/internal/errors"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/models"

	"github.com/sirupsen/logrus"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

var logger = monitor.NewModuleLogger("access_tracker")

var loc, _ = time.LoadLocation("UTC")

// TimeNow returns the current time in UTC. The only way to set the local timezone in Go is to
// set the TZ env var. Otherwise all times are created using the local timezone of your OS. This
// can screw up your time comparisons if you're not careful. Best practice: always use UTC
func TimeNow() time.Time {
	return time.Now().In(loc)
}

// Touch sets the wallet access time for a user to now
func Touch(db boil.Executor, userID int) error {
	q := fmt.Sprintf(`UPDATE "%s" SET "%s" = $1 WHERE "%s" = $2`,
		models.TableNames.Users,
		models.UserColumns.WalletAccessedAt,
		models.UserColumns.ID,
	)
	_, err := db.Exec(q, TimeNow(), userID)
	if err != nil {
		return errors.Err(err)

	}
	return nil
}

// Unload unloads wallets of users who have not accessed their wallet recently
func Unload(db boil.Executor, olderThan time.Duration) (int, error) {
	start := time.Now() // not TimeNow() because its just for checking call duration
	cutoffTime := TimeNow().Add(-olderThan)
	logger.Log().Infof("unloading wallets that were not accessed since %s", cutoffTime)

	users, err := models.Users(
		models.UserWhere.WalletAccessedAt.LT(null.TimeFrom(cutoffTime)),
		qm.Load(models.UserRels.LbrynetServer),
	).All(db)
	if err != nil {
		return 0, errors.Err(err)
	}

	for _, user := range users {
		if user.R == nil || user.R.LbrynetServer == nil {
			continue
		}
		if !user.WalletAccessedAt.Time.Before(cutoffTime) { // just in case
			continue
		}

		l := logger.WithFields(logrus.Fields{"user_id": user.ID})

		err = wallet.UnloadWallet(user.R.LbrynetServer.Address, user.ID)
		if err != nil {
			l.Error(err)
			continue
		}

		// only mark wallet unloaded if it hasn't been touched since we ran the query
		// otherwise it may never be unloaded
		q := fmt.Sprintf(`UPDATE "%s" SET "%s" = NULL WHERE "%s" = $1 AND "%s" = $2`,
			models.TableNames.Users,
			models.UserColumns.WalletAccessedAt,
			models.UserColumns.ID,
			models.UserColumns.WalletAccessedAt,
		)
		_, err := db.Exec(q, user.ID, user.WalletAccessedAt.Time)
		if err != nil {
			l.Error(err)
			continue
		}
	}

	logger.Log().Infof("unloaded %d wallets in %s", len(users), time.Since(start))
	return len(users), nil
}
