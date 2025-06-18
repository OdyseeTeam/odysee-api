package admin

import (
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/ip"
	"github.com/OdyseeTeam/odysee-api/internal/monitor"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/iprate"

	"github.com/gorilla/mux"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"golang.org/x/time/rate"
)

var logger = monitor.NewModuleLogger("proxy")
var sdkNameMatch = regexp.MustCompile(`^lbrynet\-([a-z])\-(\d+)`)

type sdkDetails struct {
	group string
	id    int
}

func InstallRoutes(r *mux.Router) error {
	limiter := iprate.NewLimiter(rate.Limit(0.05), 1, iprate.WithCleanupInterval(60*time.Minute))
	r.Use(
		SimpleAdminAuthMiddleware(config.GetSimpleAdminToken(), limiter),
	)
	r.HandleFunc("/users/{user_id}/bump-sdk", BumpSDK).Methods(http.MethodPost)
	return nil
}

func BumpSDK(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, err := strconv.Atoi(vars["user_id"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	user, err := models.Users(
		models.UserWhere.ID.EQ(userID),
		qm.Load(models.UserRels.LbrynetServer),
	).OneG()
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	origSDK := user.R.LbrynetServer
	sdkDetails, err := parseSDKDetails(origSDK.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sdkDetails.bumpID()

	newSDK, err := models.LbrynetServers(models.LbrynetServerWhere.Name.EQ(sdkDetails.String())).OneG()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sdkDetails.zeroID()
			newSDK, err = models.LbrynetServers(models.LbrynetServerWhere.Name.EQ(sdkDetails.String())).OneG()
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to select %s: %s", sdkDetails, err), http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	user.LbrynetServerID.SetValid(newSDK.ID)
	_, err = user.UpdateG(boil.Infer())
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to set sdk instance to %s: %s", sdkDetails, err), http.StatusInternalServerError)
		return
	}

	loadWalletErr := wallet.LoadWallet(newSDK.Address, user.ID)
	unloadWalletErr := wallet.UnloadWallet(origSDK.Address, user.ID)

	fmt.Fprintf(w, "bumped user %d from %s to %s", user.ID, origSDK.Name, sdkDetails)
	logger.Log().Infof("bumped user %d from %s to %s", user.ID, origSDK.Name, sdkDetails)
	if loadWalletErr != nil {
		fmt.Fprintf(w, "\nerror encountered loading wallet: %s", loadWalletErr.Error())
		logger.Log().Errorf("error loading wallet: %s", loadWalletErr)
	}
	if unloadWalletErr != nil {
		fmt.Fprintf(w, "\nerror encountered unloading wallet: %s", unloadWalletErr.Error())
		logger.Log().Errorf("error unloading wallet: %s", unloadWalletErr)
	}
}

func SimpleAdminAuthMiddleware(token string, limiter *iprate.Limiter) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				http.Error(w, "simple admin token not configured", http.StatusInternalServerError)
				return
			}

			clientIP := ip.ForRequest(r)
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				logger.Log().Errorf("failed simple auth attempt for %s", clientIP)
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			requestToken := strings.TrimPrefix(authHeader, "Bearer ")
			if subtle.ConstantTimeCompare([]byte(requestToken), []byte(token)) != 1 {
				limiter := limiter.GetLimiter(clientIP)

				if !limiter.Allow() {
					logger.Log().Errorf("failed simple auth attempt for %s, rate limited", clientIP)
					http.Error(w, "too many attempts", http.StatusTooManyRequests)
					return
				}

				logger.Log().Errorf("failed simple auth attempt for %s", clientIP)
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func parseSDKDetails(name string) (*sdkDetails, error) {
	sdkNameBits := sdkNameMatch.FindStringSubmatch(name)

	if len(sdkNameBits) != 3 {
		return nil, fmt.Errorf("weird lbrynet server name: %s (matched: %s)", name, sdkNameBits)
	}
	id, err := strconv.Atoi(sdkNameBits[2])
	if err != nil {
		return nil, fmt.Errorf("cannot parse lbrynet server name '%s': %w", name, err)
	}
	return &sdkDetails{
		group: sdkNameBits[1],
		id:    id,
	}, nil
}

func (d *sdkDetails) bumpID() {
	d.id += 1
}

func (d *sdkDetails) zeroID() {
	d.id = 0
}

func (d *sdkDetails) String() string {
	return fmt.Sprintf("lbrynet-%s-%d", d.group, d.id)
}
