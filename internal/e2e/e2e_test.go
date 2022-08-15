package e2e

import (
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OdyseeTeam/odysee-api/api"
	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/e2etest"
	"github.com/OdyseeTeam/odysee-api/internal/test"
	"github.com/OdyseeTeam/odysee-api/models"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/suite"
)

type e2eSuite struct {
	e2etest.FullSuite

	router *mux.Router
}

func (s *e2eSuite) TestProxyRoute() {
	(&test.HTTPTest{
		Method:      http.MethodPost,
		URL:         "/api/v1/proxy",
		ReqBody:     strings.NewReader(`{"method": "status"}`),
		Code:        http.StatusOK,
		ResContains: `"best_blockhash":`,
	}).Run(s.router, s.T())
}

func (s *e2eSuite) TestUpload() {

	// defer config.Config.RestoreOverridden()

	token, err := wallet.GetTestTokenHeader()
	s.Require().NoError(err)

	u, err := s.router.Get("tus_publish").URL()
	s.Require().NoError(err)
	rr := (&test.HTTPTest{
		Method: http.MethodPost,
		URL:    u.Path,
		ReqHeader: map[string]string{
			"Tus-Resumable":            "1.0.0",
			"Upload-Length":            "5",
			wallet.AuthorizationHeader: token,
		},
		ReqBody: strings.NewReader("hello"),
		Code:    http.StatusCreated,
		ResHeader: map[string]string{
			"Location": "https://buy.art/files/foo",
		},
	}).Run(s.router, s.T())

	loc, err := url.Parse(rr.Header().Get("Location"))
	s.Require().NoError(err)
	id := filepath.Base(loc.Path)

	_, err = models.Uploads(models.UploadWhere.ID.EQ(id)).One(s.DB)
	s.Require().NoError(err)

	// assert.Equal(t, http.StatusOK, rr.Code)
	// assert.Contains(t, rr.Body.String(), `"result":`)

	// rr := httptest.NewRecorder()
	// auth.LegacyMiddleware(provider)(http.HandlerFunc(handler.Handle)).ServeHTTP(rr, r)
	// response := rr.Result()
	// respBody, err := ioutil.ReadAll(response.Body)
	// require.NoError(t, err)

	// assert.Equal(t, http.StatusOK, response.StatusCode)
	// test.AssertEqualJSON(t, expectedPublishResponse, respBody)

	// require.True(t, publisher.called)
	// expectedPath := path.Join(os.TempDir(), "20404", ".+", "ody_auto_test_file")
	// assert.Regexp(t, expectedPath, publisher.filePath)
	// assert.Equal(t, sdkrouter.WalletID(20404), publisher.walletID)
	// expectedReq := fmt.Sprintf(testPublishRequest, sdkrouter.WalletID(20404), publisher.filePath)
	// test.AssertEqualJSON(t, expectedReq, publisher.rawQuery)

	// _, err = os.Stat(publisher.filePath)
	// assert.True(t, os.IsNotExist(err))
}

func (s *e2eSuite) SetupSuite() {
	s.FullSuite.SetupSuite()
	router := mux.NewRouter()
	config.Config.Override("PublishSourceDir", s.T().TempDir())
	api.InstallRoutes(router, s.SDKRouter)

	s.router = router
}

func (s *e2eSuite) TearDownSuite() {
	config.Config.RestoreOverridden()
	s.FullSuite.TearDownSuite()
}

func TestE2ESuite(t *testing.T) {
	suite.Run(t, new(e2eSuite))
}
