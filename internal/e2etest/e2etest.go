package e2etest

import (
	"database/sql"
	"errors"
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
	"github.com/stretchr/testify/suite"
)

type TestUser struct {
	User        *models.User
	SDKAddress  string
	CurrentUser *auth.CurrentUser
}

type FullSuite struct {
	suite.Suite

	TokenHeader string
	DB          *sql.DB
	SDKRouter   *sdkrouter.Router
	Auther      auth.Authenticator
	TestUser    *TestUser

	dbCleanup migrator.TestDBCleanup
}

func (s *FullSuite) SetupSuite() {
	config.Override("LbrynetServers", "")

	db, dbCleanup, err := migrator.CreateTestDB(migrator.DBConfigFromApp(config.GetDatabase()), storage.MigrationsFS)
	s.Require().NoError(err)
	storage.SetDB(db)
	s.dbCleanup = dbCleanup
	s.DB = db
	s.SDKRouter = sdkrouter.New(config.GetLbrynetServers())

	th, err := test.GetTestTokenHeader()
	s.Require().NoError(err)
	s.TokenHeader = th

	auther, err := wallet.NewOauthAuthenticator(
		config.GetOauthProviderURL(), config.GetOauthClientID(),
		config.GetInternalAPIHost(), s.SDKRouter)
	s.Require().NoError(err, errors.Unwrap(err))
	s.Auther = auther

	u, err := auther.Authenticate(s.TokenHeader, "127.0.0.1")
	s.Require().NoError(err)

	iac, err := iapi.NewClient(
		iapi.WithOAuthToken(strings.TrimPrefix(th, wallet.TokenPrefix)),
		iapi.WithRemoteIP("8.8.8.8"),
	)
	s.Require().NoError(err)

	cu := auth.NewCurrentUser(u, nil)
	cu.IP = "8.8.8.8"
	cu.IAPIClient = iac

	s.TestUser = &TestUser{
		User:        u,
		SDKAddress:  sdkrouter.GetSDKAddress(u),
		CurrentUser: cu,
	}
}

func (s *FullSuite) TearDownSuite() {
	s.dbCleanup()
	config.RestoreOverridden()
}
