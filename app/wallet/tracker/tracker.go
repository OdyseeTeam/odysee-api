package tracker

import (
	"fmt"
	"net/http"
	"time"

	"github.com/lbryio/lbrytv/app/auth"
	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/internal/errors"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/models"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

var wtLogger = monitor.NewModuleLogger("wallet_tracker")

func GetLogger() monitor.ModuleLogger { return wtLogger } // for testing

// TimeNow returns the current time in UTC. The only way to set the local timezone in Go is to
// set the TZ env var. Otherwise all times are created using the local timezone of your OS. This
// can screw up your time comparisons if you're not careful. Best practice: always use UTC
func TimeNow() time.Time {
	return time.Now().UTC()
}

// Touch sets the wallet access time for a user to now
func Touch(db boil.Executor, userID int) error {
	q := fmt.Sprintf(`UPDATE "%s" SET "%s" = $1 WHERE "%s" = $2`,
		models.TableNames.Users,
		models.UserColumns.LastSeenAt,
		models.UserColumns.ID,
	)
	_, err := db.Exec(q, TimeNow(), userID)
	if err != nil {
		return errors.Err(err)
	}
	wtLogger.WithFields(logrus.Fields{"user_id": userID}).Trace("touched user")
	return nil
}

// Unload unloads wallets of users who have not accessed their wallet recently
func Unload(db boil.Executor, olderThan time.Duration) (int, error) {
	start := time.Now() // not TimeNow() because its just for checking call duration
	cutoffTime := TimeNow().Add(-olderThan)
	wtLogger.Log().Infof("unloading wallets that were not accessed since %s", cutoffTime)

	users, err := models.Users(
		models.UserWhere.LastSeenAt.LT(null.TimeFrom(cutoffTime)),
		qm.Load(models.UserRels.LbrynetServer),
	).All(db)
	if err != nil {
		return 0, errors.Err(err)
	}

	for _, user := range users {
		if user.R == nil || user.R.LbrynetServer == nil {
			continue
		}
		if !user.LastSeenAt.Time.Before(cutoffTime) { // just in case
			continue
		}

		l := wtLogger.WithFields(logrus.Fields{"user_id": user.ID})

		err = wallet.UnloadWallet(user.R.LbrynetServer.Address, user.ID)
		if err != nil {
			l.Error(err)
			continue
		}

		// only mark wallet unloaded if it hasn't been touched since we ran the query
		// otherwise it may never be unloaded
		q := fmt.Sprintf(`UPDATE "%s" SET "%s" = NULL WHERE "%s" = $1 AND "%s" = $2`,
			models.TableNames.Users,
			models.UserColumns.LastSeenAt,
			models.UserColumns.ID,
			models.UserColumns.LastSeenAt,
		)
		_, err := db.Exec(q, user.ID, user.LastSeenAt.Time)
		if err != nil {
			l.Error(err)
			continue
		}
	}

	wtLogger.Log().Infof("unloaded %d wallets in %s", len(users), time.Since(start))
	return len(users), nil
}

func Middleware(db boil.Executor) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)

			authRes, err := auth.FromRequest(r)
			user := authRes.User
			if err != nil && !errors.Is(err, auth.ErrNoAuthInfo) {
				wtLogger.Log().Error(err)
				return
			}
			if user == nil {
				return
			}

			err = Touch(db, user.ID)
			if err != nil {
				monitor.ErrorToSentry(err)
				wtLogger.Log().Errorf("error touching wallet access time: %v", err)
			}
		})
	}
}
