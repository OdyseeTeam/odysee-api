package e2etest

import (
	"database/sql"
	"strings"

	"github.com/OdyseeTeam/odysee-api/app/auth"
	"github.com/OdyseeTeam/odysee-api/app/sdkrouter"
	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/storage"
	"github.com/OdyseeTeam/odysee-api/internal/test"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/iapi"
	"github.com/OdyseeTeam/odysee-api/pkg/migrator"
	"github.com/lbryio/transcoder/pkg/logging/zapadapter"
)

type cleanupFunc func() error

type TestUser struct {
	User        *models.User
	SDKAddress  string
	CurrentUser *auth.CurrentUser
}

type UserTestHelper struct {
	TokenHeader string
	DB          *sql.DB
	SDKRouter   *sdkrouter.Router
	Auther      auth.Authenticator
	TestUser    *TestUser

	CleanupFuncs []cleanupFunc
}

func (s *UserTestHelper) Setup() error {
	s.CleanupFuncs = []cleanupFunc{}
	config.Override("LbrynetServers", "")

	db, dbCleanup, err := migrator.CreateTestDB(migrator.DBConfigFromApp(config.GetDatabase()), storage.MigrationsFS)
	if err != nil {
		panic(err)
	}
	storage.SetDB(db)
	s.CleanupFuncs = append(s.CleanupFuncs, func() error {
		zapadapter.NewKV(nil).Info("cleaning up usertesthelper db")
		return dbCleanup()
	})
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

	cu := auth.NewCurrentUser(u, nil)
	cu.IP = "8.8.8.8"
	cu.IAPIClient = iac

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

func (s *UserTestHelper) Cleanup() {
	for _, f := range s.CleanupFuncs {
		f()
	}
	config.RestoreOverridden()
}

func (s *UserTestHelper) InjectTestingWallet() error {
	_, err := test.InjectTestingWallet(s.UserID())
	return err
}
