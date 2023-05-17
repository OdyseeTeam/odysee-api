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
	s.t = t
	config.Override("LbrynetServers", "")

	db, dbCleanup, err := migrator.CreateTestDB(migrator.DBConfigFromApp(config.GetDatabase()), storage.MigrationsFS)
	if err != nil {
		panic(err)
	}
	storage.SetDB(db)
	t.Cleanup(func() { dbCleanup() })
	s.DB = db
	s.SDKRouter = sdkrouter.New(config.GetLbrynetServers())

	th, err := test.GetTestTokenHeader()
	if err != nil {
		return err
	}
	s.TokenHeader = th

	auther, err := wallet.NewOauthAuthenticator(
		config.GetOauthProviderURL(), config.GetOauthClientID(),
		config.GetInternalAPIHost(), s.SDKRouter)
	if err != nil {
		return err
	}
	s.Auther = auther

	w, err := test.InjectTestingWallet(test.TestUserID)
	if err != nil {
		return err
	}
	s.t.Logf("set up wallet userid=%v", w.UserID)
	t.Cleanup(func() {
		w.Unload()
		w.RemoveFile()
	})

	u, err := auther.Authenticate(s.TokenHeader, "127.0.0.1")
	if err != nil {
		return err
	}

	iac, err := iapi.NewClient(
		iapi.WithOAuthToken(strings.TrimPrefix(th, wallet.TokenPrefix)),
		iapi.WithRemoteIP("8.8.8.8"),
	)
	if err != nil {
		return err
	}

	cu := auth.NewCurrentUser(u, "8.8.8.8", iac, nil)

	s.TestUser = &TestUser{
		User:        u,
		SDKAddress:  sdkrouter.GetSDKAddress(u),
		CurrentUser: cu,
	}

	t.Cleanup(config.RestoreOverridden)
	return nil
}

func (s *UserTestHelper) UserID() int {
	return s.TestUser.User.ID
}
