package wallet

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/internal/errors"
	"github.com/lbryio/lbrytv/internal/storage"
	"github.com/lbryio/lbrytv/models"

	"github.com/coreos/go-oidc"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"golang.org/x/oauth2"
)

const oauthClientID = "odysee-apis"
const oauthProviderUrl = "https://sso.odysee.com/auth/realms/Users"

var verifier *oidc.IDTokenVerifier

func init() {
	provider, err := oidc.NewProvider(context.Background(), oauthProviderUrl)
	if err != nil {
		panic(err)
	}
	verifier = provider.Verifier(&oidc.Config{ClientID: oauthClientID})
}

// GetOauthUserWithSDKServer gets user by internal-apis auth token. If the user does not have a
// wallet yet, they are assigned an SDK and a wallet is created for them on that SDK.
func GetOauthUserWithSDKServer(rt *sdkrouter.Router, internalAPIHost, tokenString, metaRemoteIP string) (*models.User, error) {
	var localUser *models.User
	if !strings.HasPrefix(tokenString, "Bearer ") {
		return nil, errors.Err("token passed must be Bearer token")
	}
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	userInfo, err := extractUserInfo(tokenString)
	if err != nil {
		return nil, err
	}

	// Todo - Would be really nice to change provider to access the http request to get and validate more information
	err = checkAuthorization(userInfo)
	if err != nil {
		return nil, err
	}

	log := logger.WithFields(logrus.Fields{"idp_user": userInfo.Sub, "ip": metaRemoteIP})

	if cachedUser := currentCache.get(userInfo.Sub); cachedUser != nil {
		log.Debugf("user found in cache")
		return cachedUser, nil
	}

	//Check if we have the user by IDP ID first
	user, err := GetDBUserG(ByIDPID(userInfo.Sub))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if user != nil {
		return user, nil
	}

	// No mention of it recorded in the DB, so get the internal-apis user ID via remote user
	remoteUser, err := getRemoteUser(internalAPIHost, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: tokenString}), metaRemoteIP)
	if err != nil {
		/*  For now keep the same process, where if we fail to get a remote user, authentication fails.
		We don't have to check for verified email since IDP already confirmed via OAuth2. This should
		be different, where we don't need a user id from internal-apis to continue but for now, to
		keep it simple lets just require it instead of changing primary keys and wallet names. Also as
		the wallets are populated with the IDP ID these calls will dwindle down to first time users.

		When we do, do it, we should populate IDP_ID for wallet_IDS for all new wallets created, storing
		all user id based wallet ids as they arise. Then we will be left with old wallets that have not
		been accessed in a long time and can be wiped. When they sign in at some point in the future
		they will get the wallet from the wallet sync APIs.
		*/
		return nil, err
	}
	log.Data["remote_user_id"] = remoteUser.ID
	log.Debugf("user id retrieved from internal-apis")

	//See if we already have a wallet but under the user ID from internal-apis
	user, err = GetDBUserG(ByID(remoteUser.ID))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if user != nil {
		user.IdpID.SetValid(userInfo.Sub)
		_, err := user.UpdateG(boil.Infer())
		if err != nil {
			return nil, errors.Err(err)
		}
		return user, nil
	}

	// Ok, no user in internal-apis either. Time to create a new one...
	ctx, cancelFn := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelFn()

	err = inTx(ctx, storage.Conn.DB.DB, func(tx *sql.Tx) error {
		localUser, err = getOrCreateLocalUser(tx, models.User{ID: remoteUser.ID, IdpID: null.StringFrom(userInfo.Sub)}, log)
		if err != nil {
			return err
		}

		if localUser.LbrynetServerID.IsZero() {
			err := assignSDKServerToUser(tx, localUser, rt.LeastLoaded(), log)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err == nil && localUser != nil {
		currentCache.set(tokenString, localUser)
	}

	return localUser, err
}

func checkAuthorization(info *UserInfo) error {
	audienceRespected := false
	for _, aud := range info.Aud {
		if aud == oauthClientID {
			audienceRespected = true
		}
	}
	if !audienceRespected {
		return errors.Err("this token is not meant for Odysee APIs")
	}

	/* If we could get the http request we could valid allowed sources
	allowedSource := false
	for _, url := range info.AllowedOrigins {
		if url == r.Host {
			allowedSource = true
		}
	}
	if !allowedSource {
		return errors.Err("this token cannot be used from %s", r.Host)
	}*/

	return nil
}

// UserInfo contains all claim information included in the access token.
type UserInfo struct {
	Acr               string              `mapstructure:"acr"`
	AllowedOrigins    []string            `json:"allowed-origins"`
	Aud               []string            `mapstructure:"aud"`
	Azp               string              `mapstructure:"azp"`
	Email             string              `mapstructure:"email"`
	EmailVerified     bool                `mapstructure:"email_verified"`
	Exp               int64               `mapstructure:"exp"`
	FamilyName        string              `mapstructure:"family_name"`
	GivenName         string              `mapstructure:"given_name"`
	Iat               int64               `mapstructure:"iat"`
	Iss               string              `mapstructure:"iss"`
	Jti               string              `mapstructure:"jti"`
	Name              string              `mapstructure:"name"`
	PreferredUsername string              `mapstructure:"preferred_username"`
	RealmAccess       map[string][]string `mapstructure:"realm_access"`
	//ResourceAccess    map[string]map[string][]string `mapstructure:"resource_access"`
	ResourceAccess struct {
		OdyseeApis struct {
			Roles []string `mapstructure:"roles"`
		} `mapstructure:"odysee-apis"`
	} `mapstructure:"resource_access"`
	Scope        string `mapstructure:"scope"`
	SessionState string `mapstructure:"session_state"`
	Sid          string `mapstructure:"sid"`
	Sub          string `mapstructure:"sub"`
	Typ          string `mapstructure:"typ"`
}

func extractUserInfo(tokenString string) (*UserInfo, error) {
	userInfo := &UserInfo{}

	t, err := verifier.Verify(context.Background(), tokenString)
	if err != nil {
		return nil, errors.Err(err)
	}
	err = t.Claims(userInfo)
	if err != nil {
		return nil, errors.Err(err)
	}

	return userInfo, nil
}
