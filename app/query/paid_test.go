package query

import (
	"context"
	"database/sql"
	"fmt"
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
	rentalClaim   = "22acd6a6ab1c83d8c265d652c3842420810006be"
	purchaseClaim = "2742f9e8eea0c4654ea8b51507dbb7f23f1f5235"

	rentalURL   = "lbry://@gifprofile#7/test-rental-2#2"
	purchaseURL = "lbry://@gifprofile#7/purchase1#2"

	fakeRemoteIP = "171.100.27.222"
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

	u, err := auther.Authenticate(s.tokenHeader, fakeRemoteIP)
	s.Require().NoError(err)
	s.user = u
	s.sdkAddress = sdkrouter.GetSDKAddress(u)

	iac, err := iapi.NewClient(
		iapi.WithOAuthToken(strings.TrimPrefix(th, wallet.TokenPrefix)),
		iapi.WithRemoteIP(fakeRemoteIP),
	)
	s.Require().NoError(err)

	cu := auth.NewCurrentUser(u, nil)
	cu.IP = fakeRemoteIP
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
		"https://secure.odycdn.com/v5/streams/start/2742f9e8eea0c4654ea8b51507dbb7f23f1f5235/2ef2a4?hash-hls=4e42be75b03ce2237e8ff8284c794392&ip=8.8.8.8&hash=910a69e8e189288c29a5695314b48e89",
		getResponse.StreamingURL,
	)
}

func (s *paidContentSuite) TestRentalAuthorized() {
	request := jsonrpc.NewRequest(MethodGet, map[string]interface{}{"uri": rentalURL})

	ctx := auth.AttachCurrentUser(bgctx(), s.cu)
	resp, err := NewCaller(s.sdkAddress, s.user.ID).Call(ctx, request)

	s.Require().NoError(err)
	s.Require().Nil(resp.Error)

	getResponse := &ljsonrpc.GetResponse{}
	err = resp.GetObject(&getResponse)
	s.Require().NoError(err)
	s.Equal(
		fmt.Sprintf(
			"https://secure.odycdn.com/v5/streams/start/%s/2ef2a4?hash-hls=4e42be75b03ce2237e8ff8284c794392&ip=%s&hash=910a69e8e189288c29a5695314b48e89",
			purchaseClaim,
			fakeRemoteIP,
		),
		getResponse.StreamingURL,
	)
}
