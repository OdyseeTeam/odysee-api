package tracker

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/lbryio/lbrytv/app/auth"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/volatiletech/sqlboiler/boil"
)

func Middleware(db boil.Executor) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)

			user, err := auth.FromRequest(r)
			if err != nil {
				logger.Log().Error(err)
				return
			}
			if user == nil {
				return
			}

			err = Touch(db, user.ID)
			if err != nil {
				monitor.ErrorToSentry(err)
				logger.Log().Errorf("error touching wallet access time: %v", err)
			}
		})
	}
}
