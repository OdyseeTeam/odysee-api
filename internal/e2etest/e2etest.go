package e2etest

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/OdyseeTeam/odysee-api/app/auth"
	"github.com/OdyseeTeam/odysee-api/app/sdkrouter"
	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/storage"
	"github.com/OdyseeTeam/odysee-api/internal/test"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/iapi"
	"github.com/OdyseeTeam/odysee-api/pkg/migrator"
	"github.com/stretchr/testify/require"
)

type TestUser struct {
	User        *models.User
	SDKAddress  string
	CurrentUser *auth.CurrentUser
}

type UserTestHelper struct {
	t           *testing.T
	TokenHeader string
	DB          *sql.DB
	SDKRouter   *sdkrouter.Router
	Auther      auth.Authenticator
	TestUser    *TestUser
}

func (s *UserTestHelper) Setup(t *testing.T) error {
	t.Helper()
	require := require.New(t)
	s.t = t
	config.Override("LbrynetServers", "")

	db, dbCleanup, err := migrator.CreateTestDB(migrator.DBConfigFromApp(config.GetDatabase()), storage.MigrationsFS)
	require.NoError(err)
	storage.SetDB(db)

	sdkr := sdkrouter.New(config.GetLbrynetServers())

	th, err := test.GetTestTokenHeader()
	require.NoError(err)

	auther, err := wallet.NewOauthAuthenticator(
		config.GetOauthProviderURL(), config.GetOauthClientID(),
		config.GetInternalAPIHost(), sdkr)
	require.NoError(err)

	w, err := test.InjectTestingWallet(test.TestUserID)
	require.NoError(err)
	t.Logf("set up wallet userid=%v", w.UserID)

	u, err := auther.Authenticate(th, "127.0.0.1")
	require.NoError(err)

	iac, err := iapi.NewClient(
		iapi.WithOAuthToken(strings.TrimPrefix(th, wallet.TokenPrefix)),
		iapi.WithRemoteIP("8.8.8.8"),
	)
	require.NoError(err)

	cu := auth.NewCurrentUser(u, "8.8.8.8", iac, nil)

	t.Cleanup(func() {
		dbCleanup()
		w.Unload()
		w.RemoveFile()
	})
	t.Cleanup(config.RestoreOverridden)

	s.Auther = auther
	s.SDKRouter = sdkr
	s.TokenHeader = th
	s.DB = db
	s.TestUser = &TestUser{
		User:        u,
		SDKAddress:  sdkrouter.GetSDKAddress(u),
		CurrentUser: cu,
	}
	return nil
}

func (s *UserTestHelper) UserID() int {
	return s.TestUser.User.ID
}
