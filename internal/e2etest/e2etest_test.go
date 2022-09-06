package e2etest

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OdyseeTeam/odysee-api/api"
	"github.com/OdyseeTeam/odysee-api/app/geopublish/forklift"
	"github.com/OdyseeTeam/odysee-api/app/query"
	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/test"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"github.com/ybbus/jsonrpc"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/suite"
)

type e2eSuite struct {
	suite.Suite

	userHelper     *UserTestHelper
	forkliftHelper *forklift.ForkliftTestHelper
	router         *mux.Router
	forkliftErr    error
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

func (s *e2eSuite) TestPublishV3() {
	if s.forkliftErr != nil {
		s.T().Skipf(s.forkliftErr.Error())
	}

	// defer config.Config.RestoreOverridden()

	fnb64 := base64.StdEncoding.EncodeToString([]byte("dummy.md"))
	f := []byte("test file")

	u, err := s.router.Get("geopublish").URL()
	s.Require().NoError(err)

	rr := (&test.HTTPTest{
		Method: http.MethodPost,
		URL:    u.Path,
		ReqHeader: map[string]string{
			"Tus-Resumable":   "1.0.0",
			"Upload-Length":   fmt.Sprintf("%d", len(f)),
			"Upload-Metadata": fmt.Sprintf("filename %s", fnb64),
			// "Upload-Offset":            "0",
			// "Content-Type":             "application/offset+octet-stream",
			wallet.AuthorizationHeader: s.userHelper.TokenHeader,
		},
		ReqBody: bytes.NewReader(f),
		Code:    http.StatusCreated,
	}).Run(s.router, s.T())
	loc, err := url.Parse(rr.Header().Get("Location"))
	s.Require().NoError(err)
	s.Regexp("/api/v3/publish/[a-z0-9]{32}", loc.RequestURI())

	id := filepath.Base(loc.Path)

	time.Sleep(2 * time.Second)
	upload, err := models.Uploads(
		models.UploadWhere.ID.EQ(id), qm.Load(models.UploadRels.PublishQuery),
	).One(s.userHelper.DB)
	s.Require().NoError(err)
	s.Equal(models.UploadStatusCreated, upload.Status)

	(&test.HTTPTest{
		Method: http.MethodPatch,
		URL:    loc.RequestURI(),
		ReqHeader: map[string]string{
			"Tus-Resumable": "1.0.0",
			// "Upload-Length":            fmt.Sprintf("%d", len(f)),
			"Upload-Offset":            "0",
			"Content-Type":             "application/offset+octet-stream",
			wallet.AuthorizationHeader: s.userHelper.TokenHeader,
		},
		ReqBody: bytes.NewReader(f),
		Code:    http.StatusNoContent,
	}).Run(s.router, s.T())
	time.Sleep(2 * time.Second)
	upload, err = models.Uploads(
		models.UploadWhere.ID.EQ(id), qm.Load(models.UploadRels.PublishQuery),
	).One(s.userHelper.DB)
	s.Require().NoError(err)
	s.Equal(models.UploadStatusUploading, upload.Status)

	req, err := json.Marshal(jsonrpc.NewRequest(query.MethodStreamCreate, map[string]interface{}{
		"name":          "publish2test-dummymd",
		"title":         "Publish v2 test for dummy.md",
		"description":   "",
		"locations":     []string{},
		"bid":           "0.01000000",
		"languages":     []string{"en"},
		"tags":          []string{"c:disable-comments"},
		"thumbnail_url": "https://thumbs.odycdn.com/92399dc6df41af6f7c61def97335dfa5.webp",
		"release_time":  1661882701,
		"blocking":      true,
		"preview":       false,
		"license":       "None",
		"channel_id":    "febc557fcfbe5c1813eb621f7d38a80bc4355085",
	}))
	s.Require().NoError(err)
	(&test.HTTPTest{
		Method: http.MethodPost,
		URL:    loc.RequestURI() + "/notify",
		ReqHeader: map[string]string{
			"Tus-Resumable":            "1.0.0",
			wallet.AuthorizationHeader: s.userHelper.TokenHeader,
		},
		ReqBody: bytes.NewReader(req),
		Code:    http.StatusAccepted,
	}).Run(s.router, s.T())

	time.Sleep(15 * time.Second)
	upload, err = models.Uploads(models.UploadWhere.ID.EQ(id), qm.Load(models.UploadRels.PublishQuery)).One(s.userHelper.DB)
	s.Require().NoError(err)
	s.Equal(models.UploadStatusFinished, upload.Status, upload.Error)
	s.Equal(models.PublishQueryStatusSucceeded, upload.R.PublishQuery.Status)

	sr := (&test.HTTPTest{
		Method: http.MethodGet,
		URL:    loc.RequestURI() + "/status",
		ReqHeader: map[string]string{
			wallet.AuthorizationHeader: s.userHelper.TokenHeader,
		},
		Code: http.StatusOK,
	}).Run(s.router, s.T())
	srb, err := ioutil.ReadAll(sr.Result().Body)
	s.Require().NoError(err)

	jr := jsonrpc.RPCResponse{}
	err = json.Unmarshal(srb, &jr)
	s.Require().NoError(err)

	jrb, _ := json.Marshal(jr.Result)
	scr := forklift.StreamCreateResponse{}
	err = json.Unmarshal(jrb, &scr)
	s.Require().NoError(err)
	s.Equal("dummy.md", scr.Outputs[0].Value.Source.Name)
	// s.Equal("publish2test", scr.Outputs[0].Name)
	s.EqualValues(strconv.Itoa(len(f)), scr.Outputs[0].Value.Source.Size)

	// t.Run("ResumeWithChunks", func(t *testing.T) {
	// 	h := newTestTusHandlerWithOauth(t, nil)
	// 	loc := newPartialUpload(t, h,
	// 		header{"Upload-Length", "6"},
	// 		header{wallet.LegacyTokenHeader, "legacyAuthToken123"},
	// 	)

	// 	testData := []byte("foobar")
	// 	b := bytes.NewReader(testData)

	// 	const chunkSize = 2
	// 	for i := 0; i < b.Len(); i += chunkSize {
	// 		t.Run(fmt.Sprintf("PatchOffset-%d", i), func(t *testing.T) {
	// 			buf := make([]byte, chunkSize)
	// 			if _, err := b.ReadAt(buf, int64(i)); err != nil {
	// 				t.Fatal(err)
	// 			}

	// 			w := httptest.NewRecorder()
	// 			r, err := http.NewRequest(http.MethodPatch, loc, bytes.NewReader(buf))
	// 			assert.Nil(t, err)

	// 			r.Header.Set(wallet.LegacyTokenHeader, "legacyAuthToken123")
	// 			r.Header.Set("Content-Type", "application/offset+octet-stream")
	// 			r.Header.Set("Upload-Offset", strconv.Itoa(i))
	// 			r.Header.Set("Tus-Resumable", tusVersion)

	// 			newTestMux(h).ServeHTTP(w, r)

	// 			resp := w.Result()
	// 			assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	// 		})
	// 	}
	// })
}

func (s *e2eSuite) SetupSuite() {
	config.Config.Override("PublishSourceDir", s.T().TempDir())
	config.Config.Override("GeoPublishSourceDir", s.T().TempDir())

	s.userHelper = &UserTestHelper{}
	s.forkliftHelper = &forklift.ForkliftTestHelper{}
	s.Require().NoError(s.userHelper.Setup())
	s.Require().NoError(s.userHelper.InjectTestingWallet())
	s.userHelper.InjectTestingWallet()
	err := s.forkliftHelper.Setup()
	if errors.Is(err, forklift.ErrMissingEnv) {
		s.forkliftErr = err
	}

	s.router = mux.NewRouter()
	api.InstallRoutes(s.router, s.userHelper.SDKRouter)
}

// func (s *e2eSuite) TearDownSuite() {
// 	s.userHelper.Cleanup()
// 	config.Config.RestoreOverridden()
// }

func TestE2ESuite(t *testing.T) {
	suite.Run(t, new(e2eSuite))
}
