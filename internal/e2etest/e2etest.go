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

type FullSuite struct {
	suite.Suite
	// cleanup func()
	TokenHeader string
	dbCleanup   migrator.TestDBCleanup
	DB          *sql.DB
	SDKRouter   *sdkrouter.Router
	Auther      auth.Authenticator
	User        *models.User
	SDKAddress  string
	CurrentUser *auth.CurrentUser
}

func (s *FullSuite) SetupSuite() {
	config.Override("LbrynetServers", "")

	// SDKRouter := SDKRouter.New(config.GetLbrynetServers())
	db, dbCleanup, err := migrator.CreateTestDB(migrator.DBConfigFromApp(config.GetDatabase()), storage.MigrationsFS)
	if err != nil {
		panic(err)
	}
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
	s.User = u
	s.SDKAddress = sdkrouter.GetSDKAddress(u)

	iac, err := iapi.NewClient(
		iapi.WithOAuthToken(strings.TrimPrefix(th, wallet.TokenPrefix)),
		iapi.WithRemoteIP("8.8.8.8"),
	)
	s.Require().NoError(err)

	CurrentUser := auth.NewCurrentUser(u, nil)
	CurrentUser.IP = "8.8.8.8"
	CurrentUser.IAPIClient = iac
	s.CurrentUser = CurrentUser
}

func (s *FullSuite) TearDownSuite() {
	s.dbCleanup()
	config.RestoreOverridden()
}
