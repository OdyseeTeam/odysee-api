package e2etest

import (
	"bytes"
	"database/sql"
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

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/suite"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"github.com/ybbus/jsonrpc/v2"
)

type publishV3Suite struct {
	suite.Suite

	userHelper     *UserTestHelper
	forkliftHelper *forklift.ForkliftTestHelper
	router         *mux.Router
	forkliftErr    error
}

func (s *publishV3Suite) TestProxyRoute() {
	(&test.HTTPTest{
		Method:      http.MethodPost,
		URL:         "/api/v1/proxy",
		ReqBody:     strings.NewReader(`{"method": "status"}`),
		Code:        http.StatusOK,
		ResContains: `"best_blockhash":`,
	}).Run(s.router, s.T())
}

func (s *publishV3Suite) TestPublish() {
	if testing.Short() {
		s.T().Skip("skipping testing in short mode")
	}
	if s.forkliftErr != nil {
		s.T().Skipf("skipping: %s", s.forkliftErr.Error())
	}

	fnb64 := base64.StdEncoding.EncodeToString([]byte("dummy.md"))
	f := []byte("test file")

	u, err := s.router.Get("geopublish").URL()
	s.Require().NoError(err)

	initResp := (&test.HTTPTest{
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
	loc, err := url.Parse(initResp.Header().Get("Location"))
	s.Require().NoError(err)
	s.Regexp("/api/v3/publish/[a-z0-9]{32}", loc.RequestURI())

	uploadID := filepath.Base(loc.Path)
	var upload *models.Upload

	Wait(s.T(), "upload settling into the database", 5*time.Second, 1000*time.Millisecond, func() error {
		upload, err = models.Uploads(
			models.UploadWhere.ID.EQ(uploadID), qm.Load(models.UploadRels.PublishQuery),
		).One(s.userHelper.DB)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrWaitContinue
		} else if err != nil {
			return err
		}
		return nil
	})
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

	Wait(s.T(), "upload settling into the database", 5*time.Second, 1000*time.Millisecond, func() error {
		upload, err = models.Uploads(
			models.UploadWhere.ID.EQ(uploadID),
			models.UploadWhere.Status.EQ(models.UploadStatusUploading),
			qm.Load(models.UploadRels.PublishQuery),
		).One(s.userHelper.DB)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrWaitContinue
		} else if err != nil {
			return err
		}
		return nil
	})

	s.Empty(upload.Path)

	(&test.HTTPTest{
		Method: http.MethodGet,
		URL:    loc.RequestURI() + "/status",
		ReqHeader: map[string]string{
			wallet.AuthorizationHeader: s.userHelper.TokenHeader,
		},
		Code: http.StatusNotFound,
	}).Run(s.router, s.T())

	streamCreateReq, err := json.Marshal(jsonrpc.NewRequest(query.MethodStreamCreate, map[string]interface{}{
		"name":                 "publish2test-dummymd",
		"title":                "Publish v2 test for dummy.md",
		"description":          "",
		"locations":            []string{},
		"bid":                  "0.01000000",
		"languages":            []string{"en"},
		"tags":                 []string{"c:disable-comments"},
		"thumbnail_url":        "https://thumbs.odycdn.com/92399dc6df41af6f7c61def97335dfa5.webp",
		"release_time":         1661882701,
		"blocking":             true,
		"preview":              false,
		"license":              "None",
		"channel_id":           "febc557fcfbe5c1813eb621f7d38a80bc4355085",
		"allow_duplicate_name": true,
	}))
	s.Require().NoError(err)

	(&test.HTTPTest{
		Method: http.MethodPost,
		URL:    loc.RequestURI() + "/notify",
		ReqHeader: map[string]string{
			"Tus-Resumable":            "1.0.0",
			wallet.AuthorizationHeader: s.userHelper.TokenHeader,
		},
		ReqBody: bytes.NewReader(streamCreateReq),
		Code:    http.StatusAccepted,
	}).Run(s.router, s.T())

	Wait(s.T(), "upload postprocessing", 15*time.Second, 1000*time.Millisecond, func() error {
		upload, err = models.Uploads(models.UploadWhere.ID.EQ(uploadID), qm.Load(models.UploadRels.PublishQuery)).One(s.userHelper.DB)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrWaitContinue
		} else if err != nil {
			return err
		}
		if upload.Status == models.UploadStatusFinished {
			s.Equal(models.PublishQueryStatusSucceeded, upload.R.PublishQuery.Status)
			return nil
		}
		return ErrWaitContinue
	})

	s.NotEmpty(upload.Path)

	statusResp := (&test.HTTPTest{
		Method: http.MethodGet,
		URL:    loc.RequestURI() + "/status",
		ReqHeader: map[string]string{
			wallet.AuthorizationHeader: s.userHelper.TokenHeader,
		},
		Code: http.StatusOK,
	}).Run(s.router, s.T())
	srb, err := ioutil.ReadAll(statusResp.Result().Body)
	s.Require().NoError(err)

	streamCreateResp := jsonrpc.RPCResponse{}
	err = json.Unmarshal(srb, &streamCreateResp)
	s.Require().NoError(err)

	jrb, _ := json.Marshal(streamCreateResp.Result)
	scr := forklift.StreamCreateResponse{}
	err = json.Unmarshal(jrb, &scr)
	s.Require().NoError(err)
	s.Equal("dummy.md", scr.Outputs[0].Value.Source.Name)
	s.EqualValues(strconv.Itoa(len(f)), scr.Outputs[0].Value.Source.Size)

}

func (s *publishV3Suite) SetupSuite() {
	config.Config.Override("PublishSourceDir", s.T().TempDir())
	config.Config.Override("GeoPublishSourceDir", s.T().TempDir())

	s.userHelper = &UserTestHelper{}
	s.forkliftHelper = &forklift.ForkliftTestHelper{}
	s.Require().NoError(s.userHelper.Setup(s.T()))
	err := s.forkliftHelper.Setup()
	if errors.Is(err, forklift.ErrMissingEnv) {
		s.forkliftErr = err
	}

	s.router = mux.NewRouter()
	api.InstallRoutes(s.router, s.userHelper.SDKRouter, &api.RoutesOptions{EnableV3Publish: true})
}

func TestE2ESuite(t *testing.T) {
	suite.Run(t, new(publishV3Suite))
}
