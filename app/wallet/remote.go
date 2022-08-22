package wallet

import (
	"errors"
	"strings"
	"time"

	"github.com/OdyseeTeam/odysee-api/internal/metrics"
	"github.com/OdyseeTeam/odysee-api/pkg/iapi"

	"golang.org/x/oauth2"
)

// remoteUser encapsulates internal-apis user data
type remoteUser struct {
	ID               int  `json:"user_id"`
	HasVerifiedEmail bool `json:"has_verified_email"`
	Cached           bool
}

func GetRemoteUserLegacy(server, token string, remoteIP string) (remoteUser, error) {
	ruser := remoteUser{}
	op := metrics.StartOperation(opName, "get_remote_user")
	defer op.End()

	iac, err := iapi.NewClient(
		iapi.WithLegacyToken(token),
		iapi.WithRemoteIP(remoteIP),
		iapi.WithServer(server),
	)
	if err != nil {
		return ruser, err
	}

	start := time.Now()
	resp := &iapi.UserHasVerifiedEmailResponse{}
	err = iac.Call("user/has_verified_email", map[string]string{}, resp)
	duration := time.Since(start).Seconds()

	if err != nil {
		if errors.Is(err, iapi.APIError) {
			metrics.IAPIAuthFailedDurations.Observe(duration)
		} else {
			metrics.IAPIAuthErrorDurations.Observe(duration)
		}
		return remoteUser{}, err
	}

	metrics.IAPIAuthSuccessDurations.Observe(duration)

	ruser.ID = resp.Data.UserID
	ruser.HasVerifiedEmail = resp.Data.HasVerifiedEmail

	return ruser, nil
}

func GetRemoteUser(token oauth2.TokenSource, remoteIP string) (remoteUser, error) {
	ruser := remoteUser{}
	op := metrics.StartOperation(opName, "get_remote_user")
	defer op.End()

	t, err := token.Token()
	if err != nil {
		return ruser, err
	}
	if t.Type() != "Bearer" {
		return ruser, errors.New("internal-apis requires an oAuth token of type 'Bearer'")
	}

	iac, err := iapi.NewClient(
		iapi.WithOAuthToken(strings.TrimPrefix(t.AccessToken, TokenPrefix)),
		iapi.WithRemoteIP(remoteIP),
	)
	if err != nil {
		return ruser, err
	}

	start := time.Now()
	resp := &iapi.UserHasVerifiedEmailResponse{}
	err = iac.Call("user/has_verified_email", map[string]string{}, resp)
	duration := time.Since(start).Seconds()

	if err != nil {
		if errors.Is(err, iapi.APIError) {
			metrics.IAPIAuthFailedDurations.Observe(duration)
		} else {
			metrics.IAPIAuthErrorDurations.Observe(duration)
		}
		return remoteUser{}, err
	}

	metrics.IAPIAuthSuccessDurations.Observe(duration)

	ruser.ID = resp.Data.UserID
	ruser.HasVerifiedEmail = resp.Data.HasVerifiedEmail

	return ruser, nil
}
