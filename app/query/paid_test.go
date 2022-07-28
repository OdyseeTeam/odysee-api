package query

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/OdyseeTeam/odysee-api/app/auth"
	"github.com/OdyseeTeam/odysee-api/app/sdkrouter"
	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/errors"
	"github.com/OdyseeTeam/odysee-api/internal/storage"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/iapi"
	"github.com/OdyseeTeam/odysee-api/pkg/migrator"
	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
	"github.com/ybbus/jsonrpc"

	"github.com/stretchr/testify/suite"
)

const (
	rentalClaim   = "81b1749f773bad5b9b53d21508051560f2746cdc"
	purchaseClaim = "2742f9e8eea0c4654ea8b51507dbb7f23f1f5235"

	rentalURL   = "lbry://@gifprofile#7/rental1#8"
	purchaseURL = "lbry://@gifprofile#7/purchase1#2"
)

type paidContentSuite struct {
	suite.Suite
	// cleanup func()
	tokenHeader string
	dbCleanup   migrator.TestDBCleanup
	db          *sql.DB
	sdkRouter   *sdkrouter.Router
	auther      auth.Authenticator
	user        *models.User
	sdkAddress  string
	cu          *auth.CurrentUser
}

func TestPaidContentSuite(t *testing.T) {
	suite.Run(t, new(paidContentSuite))
}

func (s *paidContentSuite) SetupSuite() {
	config.Override("LbrynetServers", "")

	// sdkRouter := sdkrouter.New(config.GetLbrynetServers())
	db, dbCleanup, err := migrator.CreateTestDB(migrator.DBConfigFromApp(config.GetDatabase()), storage.MigrationsFS)
	if err != nil {
		panic(err)
	}
	storage.SetDB(db)
	s.dbCleanup = dbCleanup
	s.db = db
	s.sdkRouter = sdkrouter.New(config.GetLbrynetServers())

	th, err := wallet.GetTestTokenHeader()
	s.Require().NoError(err)
	s.tokenHeader = th

	auther, err := wallet.NewOauthAuthenticator(
		config.GetOauthProviderURL(), config.GetOauthClientID(),
		config.GetInternalAPIHost(), s.sdkRouter)
	s.Require().NoError(err, errors.Unwrap(err))
	s.auther = auther

	u, err := auther.Authenticate(s.tokenHeader, "127.0.0.1")
	s.Require().NoError(err)
	s.user = u
	s.sdkAddress = sdkrouter.GetSDKAddress(u)

	iac, err := iapi.NewClient(
		iapi.WithOAuthToken(strings.TrimPrefix(th, wallet.TokenPrefix)),
		iapi.WithRemoteIP("8.8.8.8"),
	)
	s.Require().NoError(err)

	cu := auth.NewCurrentUser(u, nil)
	cu.IP = "8.8.8.8"
	cu.IAPIClient = iac
	s.cu = cu
}

func (s *paidContentSuite) TearDownSuite() {
	s.dbCleanup()
	config.RestoreOverridden()
}

func (s *paidContentSuite) TestPurchaseUnauthorized() {
	request := jsonrpc.NewRequest(MethodGet, map[string]interface{}{"uri": purchaseURL})

	_, err := NewCaller(s.sdkAddress, 0).Call(context.Background(), request)
	s.Error(err, "authentication required")
}

func (s *paidContentSuite) TestPurchaseAuthorized() {
	request := jsonrpc.NewRequest(MethodGet, map[string]interface{}{"uri": purchaseURL})

	ctx := auth.AttachCurrentUser(bgctx(), s.cu)
	resp, err := NewCaller(s.sdkAddress, s.user.ID).Call(ctx, request)

	s.Require().NoError(err)
	s.Require().Nil(resp.Error)

	getResponse := &ljsonrpc.GetResponse{}
	err = resp.GetObject(&getResponse)
	s.Require().NoError(err)
	s.Equal(
		"https://secure.odycdn.com/v5/streams/start/2742f9e8eea0c4654ea8b51507dbb7f23f1f5235/2ef2a4?hash-hls=55d680a5278e69a82f91e3998097e439&ip=8.8.8.8&hash=e07a49f48656c2a6cc2501d73199d192",
		getResponse.StreamingURL,
	)
}
