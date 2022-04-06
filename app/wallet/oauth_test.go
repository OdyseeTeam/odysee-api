package wallet

import (
	"context"
	"errors"
	"testing"

	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/apps/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/test"
	"github.com/lbryio/lbrytv/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestOauthAuthenticatorAuthenticate(t *testing.T) {
	setupTest()
	srv := test.RandServerAddress(t)
	rt := sdkrouter.New(map[string]string{"a": srv})
	_, cleanup := dummyAPI(srv)
	defer cleanup()

	auther, err := NewOauthAuthenticator(config.GetOauthProviderURL(), config.GetOauthClientID(), config.GetInternalAPIHost(), rt)
	require.NoError(t, err, errors.Unwrap(err))

	token, err := GetTestToken()
	require.NoError(t, err, errors.Unwrap(err))

	u, err := auther.Authenticate(token.AccessToken, "")
	require.NoError(t, err, errors.Unwrap(err))

	count, err := models.Users(models.UserWhere.ID.EQ(u.ID)).CountG()
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
	assert.True(t, u.LbrynetServerID.IsZero()) // because the server came from a config, it should not have an id set

	// now assign the user a new server thats set in the db
	//      rand.Intn(99999),
	sdk := &models.LbrynetServer{
		Name:    "testing",
		Address: "test.test.test.test",
	}
	err = u.SetLbrynetServerG(true, sdk)
	require.NoError(t, err)
	require.NotEqual(t, 0, sdk.ID)
	require.Equal(t, u.LbrynetServerID.Int, sdk.ID)

	// now fetch it all back from the db
	u2, err := auther.Authenticate(token.AccessToken, "")
	require.NoError(t, err, errors.Unwrap(err))
	require.NotNil(t, u2)

	sdk2, err := u.LbrynetServer().OneG()
	require.NoError(t, err)
	require.Equal(t, sdk.ID, sdk2.ID)
	require.Equal(t, sdk.Address, sdk2.Address)
	require.Equal(t, u.LbrynetServerID.Int, sdk2.ID)
}

func TestGetTestToken(t *testing.T) {
	token, err := GetTestToken()
	require.NoError(t, err)

	userInfo := &UserInfo{}

	auther, err := NewOauthAuthenticator(config.GetOauthProviderURL(), config.GetOauthClientID(), config.GetInternalAPIHost(), nil)
	require.NoError(t, err)
	ot, err := auther.verifier.Verify(context.Background(), token.AccessToken)
	require.NoError(t, err)

	err = ot.Claims(userInfo)
	require.NoError(t, err)

	remoteUser, err := getRemoteUser(auther.iapiURL, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token.AccessToken}), "")
	require.NoError(t, err)
	require.Greater(t, remoteUser.ID, 0)
}
