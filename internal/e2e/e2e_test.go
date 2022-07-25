package e2e

import (
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OdyseeTeam/odysee-api/api"
	"github.com/OdyseeTeam/odysee-api/app/sdkrouter"
	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/storage"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/migrator"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type e2eSuite struct {
	suite.Suite

	router    *mux.Router
	dbCleanup migrator.TestDBCleanup
	db        *sql.DB
}

type httpTest struct {
	Name string

	Method string
	URL    string

	ReqBody   io.Reader
	ReqHeader map[string]string

	Code        int
	ResBody     string
	ResHeader   map[string]string
	ResContains string
}

func (test *httpTest) Run(handler http.Handler, t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	req, err := http.NewRequest(test.Method, test.URL, test.ReqBody)
	require.NoError(t, err)
	// req.RequestURI = test.URL

	// Add headers
	for key, value := range test.ReqHeader {
		req.Header.Set(key, value)
	}

	req.Host = "odysee.com"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != test.Code {
		t.Errorf("Expected %v %s as status code (got %v %s)", test.Code, http.StatusText(test.Code), w.Code, http.StatusText(w.Code))
	}

	for key, value := range test.ResHeader {
		header := w.Header().Get(key)

		if value != header {
			t.Errorf("Expected '%s' as '%s' (got '%s')", value, key, header)
		}
	}

	if test.ResBody != "" && w.Body.String() != test.ResBody {
		t.Errorf("Expected '%s' as body (got '%s'", test.ResBody, w.Body.String())
	}

	if test.ResContains != "" && !strings.Contains(w.Body.String(), test.ResContains) {
		t.Errorf("Expected '%s' to be present in response (got '%s'", test.ResContains, w.Body.String())
	}

	return w
}

func (s *e2eSuite) SetupSuite() {
	router := mux.NewRouter()
	sdkRouter := sdkrouter.New(config.GetLbrynetServers())
	config.Config.Override("PublishSourceDir", s.T().TempDir())
	api.InstallRoutes(router, sdkRouter)

	db, dbCleanup, err := migrator.CreateTestDB(migrator.DBConfigFromApp(config.GetDatabase()), storage.MigrationsFS)
	if err != nil {
		panic(err)
	}
	storage.SetDB(db)

	s.dbCleanup = dbCleanup
	s.router = router
	s.db = db
}

func (s *e2eSuite) TearDownSuite() {
	config.Config.RestoreOverridden()
	s.dbCleanup()
}

func (s *e2eSuite) TestProxyRoute() {
	(&httpTest{
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
	rr := (&httpTest{
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

	_, err = models.Uploads(models.UploadWhere.ID.EQ(id)).One(s.db)
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

func TestE2ESuite(t *testing.T) {
	suite.Run(t, new(e2eSuite))
}
