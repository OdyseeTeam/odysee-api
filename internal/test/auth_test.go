package test

import (
	"testing"

	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestGetTestToken(t *testing.T) {
	token, err := GetTestToken()
	require.NoError(t, err)

	_, err = wallet.NewOauthAuthenticator(config.GetOauthProviderURL(), config.GetOauthClientID(), config.GetInternalAPIHost(), nil)
	require.NoError(t, err)

	remoteUser, err := wallet.GetRemoteUser(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token.AccessToken}), "")
	require.NoError(t, err)
	require.Greater(t, remoteUser.ID, 0)
}
