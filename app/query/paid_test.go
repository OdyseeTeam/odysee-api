package query

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/OdyseeTeam/odysee-api/app/auth"
	"github.com/OdyseeTeam/odysee-api/app/sdkrouter"
	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/errors"
	"github.com/OdyseeTeam/odysee-api/internal/storage"
	"github.com/OdyseeTeam/odysee-api/internal/test"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/iapi"
	"github.com/OdyseeTeam/odysee-api/pkg/migrator"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
	"github.com/stretchr/testify/suite"
	"github.com/ybbus/jsonrpc"
)

const (
	urlRental      = "lbry://@gifprofile#7/rental1#8"
	urlPurchase    = "lbry://@gifprofile#7/purchase1#2"
	urlMembersOnly = "lbry://@gifprofile#7/members-only#7"

	urlRentalExpired = "lbry://@gifprofile#7/2222222222222#8"

	urlRentalActive = "lbry://@gifprofile#7/test-rental-2#2"

	urlNoAccessPaid        = "lbry://@PlayNice#4/Alexswar#c"
	urlNoAccessMembersOnly = "lbry://@gifprofile#7/members-only-no-access#8"

	urlLivestream = "lbry://@gifprofile#7/members-only-live-2#4"

	urlV2PurchaseRental = "lbry://@gifprofile#7/purchase-and-rental-testnew#9"

	falseIP = "8.8.8.8"
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

	th, err := test.GetTestTokenHeader()
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
		iapi.WithRemoteIP(falseIP),
	)
	s.Require().NoError(err)

	cu := auth.NewCurrentUser(
		u, falseIP, iac, nil)
	s.cu = cu
}

func (s *paidContentSuite) TearDownSuite() {
	s.dbCleanup()
	config.RestoreOverridden()
}

func (s *paidContentSuite) TestUnauthorized() {
	cases := []struct {
		url, errString string
	}{
		{urlRental, "authentication required"},
		{urlPurchase, "authentication required"},
		{urlMembersOnly, "authentication required"},
		{urlLivestream, "authentication required"},
		{urlV2PurchaseRental, "authentication required"},
	}
	for _, tc := range cases {
		s.Run(tc.url, func() {
			request := jsonrpc.NewRequest(MethodGet, map[string]interface{}{"uri": tc.url, iapi.ParamEnviron: iapi.EnvironTest})
			ctx := auth.AttachCurrentUser(bgctx(), auth.NewCurrentUser(nil, falseIP, nil, errors.Err("anonymous")))
			_, err := NewCaller(s.sdkAddress, 0).Call(ctx, request)
			s.EqualError(err, tc.errString)
		})
	}
}

func (s *paidContentSuite) TestNoAccess() {
	cases := []struct {
		url, errString string
	}{
		{urlNoAccessPaid, "no access to paid content"},
		{urlRentalExpired, "rental expired"},
		{urlNoAccessMembersOnly, "no access to members-only content"},
	}
	for _, tc := range cases {
		s.Run(tc.url, func() {
			request := jsonrpc.NewRequest(MethodGet, map[string]interface{}{"uri": tc.url, iapi.ParamEnviron: iapi.EnvironTest})
			ctx := auth.AttachCurrentUser(bgctx(), s.cu)
			resp, err := NewCaller(s.sdkAddress, s.user.ID).Call(ctx, request)
			s.EqualError(err, tc.errString)
			s.Nil(resp)
		})
	}
}

func (s *paidContentSuite) TestAccess() {
	sp := "https://secure.odycdn.com/v5/streams/start"
	cases := []struct {
		url, expectedStreamingUrl string
		baseStreamingURL          string
	}{
		{url: urlRentalActive, expectedStreamingUrl: sp + "/22acd6a6ab1c83d8c265d652c3842420810006be/96a3e2?hash-hls=13545307db94557076f7588baa662e8d&ip=8.8.8.8&hash=d3aee92e2e9161a17465ff1b8c05843f"},
		{url: urlPurchase, expectedStreamingUrl: sp + "/2742f9e8eea0c4654ea8b51507dbb7f23f1f5235/2ef2a4?hash-hls=a43753e2392f2013f99e23b0d4dcc693&ip=8.8.8.8&hash=d3092ba947df383cd7bd407ff60a57e6"},
		{url: urlMembersOnly, expectedStreamingUrl: sp + "/7de672e799d17fc562ae7b381db1722a81856410/ad42aa?hash-hls=be111832e6ecda4593a9372292f07700&ip=8.8.8.8&hash=c248aa3284e0b3e27f7f0429269585df"},
		{url: urlV2PurchaseRental, expectedStreamingUrl: sp + "/970deae1469f2b4c7cc7286793b82676053ab3cd/2c2b26?hash-hls=cd78493472d31764f688009042b044ca&ip=8.8.8.8&hash=be3b8b078fce36d8ce07250fc0ddda50"},
		{
			url:                  urlLivestream,
			baseStreamingURL:     "https://cloud.odysee.live/secure/content/f9660d617e226959102e84436533638858d0b572/master.m3u8",
			expectedStreamingUrl: "https://cloud.odysee.live/secure/content/f9660d617e226959102e84436533638858d0b572/master.m3u8?ip=8.8.8.8&hash=414505d9387c3809b11229bc3e238c62",
		},
	}
	for _, tc := range cases {
		s.Run(tc.url, func() {
			params := map[string]interface{}{"uri": tc.url, iapi.ParamEnviron: iapi.EnvironTest}
			if tc.baseStreamingURL != "" {
				params[ParamBaseStreamingUrl] = tc.baseStreamingURL
			}
			request := jsonrpc.NewRequest(MethodGet, params)

			ctx := auth.AttachCurrentUser(bgctx(), s.cu)
			resp, err := NewCaller(s.sdkAddress, s.user.ID).Call(ctx, request)

			s.Require().NoError(err)
			s.Require().Nil(resp.Error)

			gresp := &ljsonrpc.GetResponse{}
			err = resp.GetObject(&gresp)
			s.Require().NoError(err)
			s.Equal(tc.expectedStreamingUrl, gresp.StreamingURL)
		})
	}
}
