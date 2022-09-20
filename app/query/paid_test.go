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
	rentalURL      = "lbry://@gifprofile#7/rental1#8"
	purchaseURL    = "lbry://@gifprofile#7/purchase1#2"
	membersOnlyURL = "lbry://@gifprofile#7/members-only#7"

	expiredRentalURL = "lbry://@gifprofile#7/2222222222222#8"
	activeRentalURL  = "lbry://@gifprofile#7/test-rental-2#2"

	noAccessPaidURL        = "lbry://@PlayNice#4/Alexswar#c"
	noAccessMembersOnlyURL = "lbry://@gifprofile#7/members-only-no-access#8"

	livestreamURL = "lbry://@gifprofile#7/members-only-livestream#f"

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

	cu := auth.NewCurrentUser(u, nil)
	cu.IP = falseIP
	cu.IAPIClient = iac
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
		{rentalURL, "authentication required"},
		{purchaseURL, "authentication required"},
		{membersOnlyURL, "authentication required"},
		{livestreamURL, "authentication required"},
	}
	for _, tc := range cases {
		s.Run(tc.url, func() {
			request := jsonrpc.NewRequest(MethodGet, map[string]interface{}{"uri": tc.url})
			ctx := auth.AttachCurrentUser(bgctx(), auth.NewCurrentUser(nil, errors.Err("anonymous")))
			_, err := NewCaller(s.sdkAddress, 0).Call(ctx, request)
			s.EqualError(err, tc.errString)
		})
	}
}

func (s *paidContentSuite) TestNoAccess() {
	cases := []struct {
		url, errString string
	}{
		{noAccessPaidURL, "no access to paid content"},
		{expiredRentalURL, "rental expired"},
		{noAccessMembersOnlyURL, "no access to members-only content"},
	}
	for _, tc := range cases {
		s.Run(tc.url, func() {
			request := jsonrpc.NewRequest(MethodGet, map[string]interface{}{"uri": tc.url})
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
		{url: activeRentalURL, expectedStreamingUrl: sp + "/22acd6a6ab1c83d8c265d652c3842420810006be/96a3e2?hash-hls=33c2dc5a5aaf863e469488009b9164a6&ip=8.8.8.8&hash=90c0a6f1859842493354b462cc857c0c"},
		{url: purchaseURL, expectedStreamingUrl: sp + "/2742f9e8eea0c4654ea8b51507dbb7f23f1f5235/2ef2a4?hash-hls=4e42be75b03ce2237e8ff8284c794392&ip=8.8.8.8&hash=910a69e8e189288c29a5695314b48e89"},
		{url: membersOnlyURL, expectedStreamingUrl: sp + "/7de672e799d17fc562ae7b381db1722a81856410/ad42aa?hash-hls=5e25826a1957b73084e85e5878fef08b&ip=8.8.8.8&hash=bcc9a904ae8621e910427f2eb3637be7"},
		{
			url:                  livestreamURL,
			baseStreamingURL:     "https://cloud.odysee.live/secure/content/f9660d617e226959102e84436533638858d0b572/master.m3u8",
			expectedStreamingUrl: "https://cloud.odysee.live/secure/content/f9660d617e226959102e84436533638858d0b572/master.m3u8?ip=8.8.8.8&hash=414505d9387c3809b11229bc3e238c62",
		},
	}
	for _, tc := range cases {
		s.Run(tc.url, func() {
			params := map[string]interface{}{"uri": tc.url}
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
