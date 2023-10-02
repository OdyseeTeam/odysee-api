package query

import (
	"context"
	"database/sql"
	"log"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	"github.com/OdyseeTeam/player-server/pkg/paid"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/ybbus/jsonrpc"
)

const (
	urlRental              = "@gifprofile#7/rental1#8"
	urlPurchase            = "@gifprofile#7/purchase1#2"
	urlMembersOnly         = "@gifprofile#7/members-only#7"
	urlRentalExpired       = "@gifprofile#7/2222222222222#8"
	urlRentalActive        = "@gifprofile#7/test-rental-2#2"
	urlNoAccessPaid        = "@PlayNice#4/Alexswar#c"
	urlNoAccessMembersOnly = "@gifprofile#7/members-only-no-access#8"
	urlLivestream          = "@gifprofile#7/members-only-live-2#4"
	urlV2PurchaseRental    = "@gifprofile#7/purchase-and-rental-testnew#9"
	urlLbcPurchase         = "test#467c565bfa083a8a784f4b9f8019e42356751955"

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

	_, err = test.InjectBuyerWallet(test.TestUserID)
	s.Require().NoError(err)

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

	err = paid.GeneratePrivateKey()
	if err != nil {
		log.Fatal(err)
	}

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
	pcfg := config.GetStreamsV6()
	host := pcfg["paidhost"]
	token := pcfg["token"]

	timeSource = riggedTimeSource{time.Now()}
	defer func() { timeSource = realTimeSource{} }()

	simpleSign := func(host, path string) string {
		u, err := signStreamURL77(host, path, token, timeSource.Now().Add(24*time.Hour).Unix())
		s.Require().NoError(err)
		return u
	}

	cases := []struct {
		url, needUrl string
		baseURL      string
	}{
		{
			url:     urlRentalActive,
			needUrl: simpleSign(host, "/v6/streams/22acd6a6ab1c83d8c265d652c3842420810006be/96a3e2/start"),
		},
		{

			url:     urlPurchase,
			needUrl: simpleSign(host, "/v6/streams/2742f9e8eea0c4654ea8b51507dbb7f23f1f5235/2ef2a4/start"),
		},
		{
			url:     urlMembersOnly,
			needUrl: simpleSign(host, "/v6/streams/7de672e799d17fc562ae7b381db1722a81856410/ad42aa/start"),
		},
		{
			url:     urlV2PurchaseRental,
			needUrl: simpleSign(host, "/v6/streams/970deae1469f2b4c7cc7286793b82676053ab3cd/2c2b26/start"),
		},
		{
			url:     urlLivestream,
			baseURL: "https://cloud.odysee.live/content/f9660d617e226959102e84436533638858d0b572/master.m3u8",
			needUrl: simpleSign("cloud.odysee.live", "/content/f9660d617e226959102e84436533638858d0b572/master.m3u8"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.url, func() {
			params := map[string]interface{}{"uri": tc.url, iapi.ParamEnviron: iapi.EnvironTest}
			if tc.baseURL != "" {
				params[ParamBaseStreamingUrl] = tc.baseURL
			}
			request := jsonrpc.NewRequest(MethodGet, params)

			ctx := auth.AttachCurrentUser(bgctx(), s.cu)
			resp, err := NewCaller(s.sdkAddress, s.user.ID).Call(ctx, request)

			s.Require().NoError(err)
			s.Require().Nil(resp.Error)

			gresp := &ljsonrpc.GetResponse{}
			err = resp.GetObject(&gresp)
			s.Require().NoError(err)
			s.Equal(tc.needUrl, gresp.StreamingURL)
		})
	}
}

func (s *paidContentSuite) TestAccessLBC() {
	params := map[string]interface{}{"uri": urlLbcPurchase}
	request := jsonrpc.NewRequest(MethodGet, params)

	ctx := auth.AttachCurrentUser(bgctx(), s.cu)
	resp, err := NewCaller(s.sdkAddress, s.user.ID).Call(ctx, request)

	s.Require().NoError(err)
	s.Require().Nil(resp.Error)

	gresp := &ljsonrpc.GetResponse{}
	err = resp.GetObject(&gresp)
	s.Require().NoError(err)

	u, err := url.Parse(gresp.StreamingURL)
	s.Require().NoError(err)

	s.Require().NoError(paid.VerifyStreamAccess(strings.Replace(urlLbcPurchase, "#", "/", 1), filepath.Base(u.Path)))

	claim := s.getClaim(urlLbcPurchase)
	s.Require().Nil(claim.PurchaseReceipt)
}

func (s *paidContentSuite) getClaim(url string) *ljsonrpc.Claim {
	c := NewCaller(s.sdkAddress, s.user.ID)
	q, err := NewQuery(jsonrpc.NewRequest("get", map[string]interface{}{
		"wallet_id": sdkrouter.WalletID(s.user.ID),
	}), sdkrouter.WalletID(s.user.ID))
	s.Require().NoError(err)

	claim, err := resolve(context.Background(), c, q, url)
	s.Require().NoError(err)
	return claim
}

func TestSignStreamURL77(t *testing.T) {
	cdnResourceURL := "player.odycdn.com"
	filePath := "/api/v4/streams/tc/all-the-times-we-nearly-blew-up-the" +
		"/ac809d68d201e2f58dcd241b5aaeefe817634dda" +
		"/2f562bd1dd318db726014d255c3c7f4e5cae3e746f77647e00ad7e9b272d193bcad634b515bf0a2bc471719cfdde0c00" +
		"/master.m3u8"
	secureToken := "aiphaechiSietee3heiKaezosaitip0i"
	expiryTimestamp := int64(1695977338)
	signedURLPath, err := signStreamURL77(cdnResourceURL, filePath, secureToken, expiryTimestamp)
	require.NoError(t, err)
	require.Equal(t, "https://player.odycdn.com/Syc1EWOyivHWw9L4aquM1g==,1695977338"+filePath, signedURLPath)
}
