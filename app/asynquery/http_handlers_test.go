package asynquery

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/auth"
	"github.com/OdyseeTeam/odysee-api/app/query"
	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/internal/e2etest"
	"github.com/OdyseeTeam/odysee-api/internal/test"
	"github.com/OdyseeTeam/odysee-api/internal/testdeps"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/keybox"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"
	"github.com/Pallinder/go-randomdata"
	"github.com/gorilla/mux"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"github.com/ybbus/jsonrpc"

	"github.com/stretchr/testify/suite"
)

type asynqueryHandlerSuite struct {
	suite.Suite

	userHelper *e2etest.UserTestHelper
	router     *mux.Router
	launcher   *Launcher
}

func TestAsynqueryHandlerSuite(t *testing.T) {
	suite.Run(t, new(asynqueryHandlerSuite))
}

func (s *asynqueryHandlerSuite) TestRetrieveUploadToken() {
	ts := httptest.NewServer(s.router)

	resp := (&test.HTTPTest{
		Method: http.MethodPost,
		URL:    ts.URL + "/api/v1/asynqueries/auth/upload-token",
		ReqHeader: map[string]string{
			wallet.AuthorizationHeader: s.userHelper.TokenHeader,
		},
		Code: http.StatusOK,
	}).Run(s.router, s.T())

	s.Equal(UploadServiceURL, resp.Header().Get("Location"))
	rr := &Response{}
	s.Require().NoError(json.Unmarshal(resp.Body.Bytes(), rr))
	s.Empty(rr.Error)
	s.Require().Equal(StatusProceed, rr.Status)
	s.NotEmpty(rr.Payload.(UploadTokenResponse).Token)
	s.Equal(UploadServiceURL, rr.Payload.(UploadTokenResponse).Location)
}

func (s *asynqueryHandlerSuite) TestCreate() {
	ts := httptest.NewServer(s.router)
	uploadID := randomdata.Alphanumeric(64)

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
		FilePathParam:          "https://uploads-v4.api.na-backend.odysee.com/v1/uploads/" + uploadID,
	}))
	s.Require().NoError(err)

	resp := (&test.HTTPTest{
		Method: http.MethodPost,
		URL:    ts.URL + "/api/v1/asynqueries/",
		ReqHeader: map[string]string{
			wallet.AuthorizationHeader: s.userHelper.TokenHeader,
		},
		ReqBody: bytes.NewReader(streamCreateReq),
		Code:    http.StatusCreated,
	}).Run(s.router, s.T())
	loc, err := url.Parse(resp.Header().Get("Location"))
	s.Require().NoError(err)
	s.Regexp(`./[\w\d]{32}`, loc.Path)

	var query *models.Asynquery
	e2etest.Wait(s.T(), "upload settling into the database", 5*time.Second, 1000*time.Millisecond, func() error {
		mods := []qm.QueryMod{
			models.AsynqueryWhere.UploadID.EQ(uploadID),
			models.AsynqueryWhere.UserID.EQ(s.userHelper.UserID()),
		}
		query, err = models.Asynqueries(mods...).One(s.launcher.db)
		if errors.Is(err, sql.ErrNoRows) {
			return e2etest.ErrWaitContinue
		} else if err != nil {
			return err
		}
		return nil
	})
	s.Equal(models.AsynqueryStatusReceived, query.Status)
	s.Equal(uploadID, query.UploadID)

	(&test.HTTPTest{
		Method: http.MethodGet,
		URL:    ts.URL + "/api/v1/asynqueries/" + query.ID,
		Code:   http.StatusUnauthorized,
	}).Run(s.router, s.T())

	(&test.HTTPTest{
		Method: http.MethodGet,
		URL:    ts.URL + "/api/v1/asynqueries/" + query.ID,
		ReqHeader: map[string]string{
			wallet.AuthorizationHeader: s.userHelper.TokenHeader,
		},
		Code: http.StatusOK,
	}).Run(s.router, s.T())

	// var rr *StreamCreateResponse
	// s.Require().NoError(json.Unmarshal(resp.Body.Bytes(), rr))

}

func (s *asynqueryHandlerSuite) SetupSuite() {
	s.userHelper = &e2etest.UserTestHelper{}
	s.Require().NoError(s.userHelper.Setup(s.T()))
	s.router = mux.NewRouter()

	kf, err := keybox.GenerateKeyfob()
	s.Require().NoError(err)

	redisHelper := testdeps.NewRedisTestHelper(s.T())
	s.launcher = NewLauncher(
		WithBusRedisOpts(redisHelper.AsynqOpts),
		WithLogger(zapadapter.NewKV(nil)),
		WithPrivateKey(kf.PrivateKey()),
		WithDB(s.userHelper.DB),
	)
	s.router.Use(auth.Middleware(s.userHelper.Auther))

	err = s.launcher.InstallRoutes(s.router)
	s.Require().NoError(err)

	s.T().Cleanup(s.launcher.Shutdown)
}

type StreamCreateResponse struct {
	Height int    `json:"height"`
	Hex    string `json:"hex"`
	Inputs []struct {
		Address       string `json:"address"`
		Amount        string `json:"amount"`
		Confirmations int    `json:"confirmations"`
		Height        int    `json:"height"`
		Nout          int    `json:"nout"`
		Timestamp     int    `json:"timestamp"`
		Txid          string `json:"txid"`
		Type          string `json:"type"`
	} `json:"inputs"`
	Outputs []struct {
		Address       string `json:"address"`
		Amount        string `json:"amount"`
		ClaimID       string `json:"claim_id,omitempty"`
		ClaimOp       string `json:"claim_op,omitempty"`
		Confirmations int    `json:"confirmations"`
		Height        int    `json:"height"`
		Meta          struct {
		} `json:"meta,omitempty"`
		Name           string      `json:"name,omitempty"`
		NormalizedName string      `json:"normalized_name,omitempty"`
		Nout           int         `json:"nout"`
		PermanentURL   string      `json:"permanent_url,omitempty"`
		Timestamp      interface{} `json:"timestamp"`
		Txid           string      `json:"txid"`
		Type           string      `json:"type"`
		Value          struct {
			Source struct {
				Hash      string `json:"hash"`
				MediaType string `json:"media_type"`
				Name      string `json:"name"`
				SdHash    string `json:"sd_hash"`
				Size      string `json:"size"`
			} `json:"source"`
			StreamType string `json:"stream_type"`
		} `json:"value,omitempty"`
		ValueType string `json:"value_type,omitempty"`
	} `json:"outputs"`
	TotalFee    string `json:"total_fee"`
	TotalInput  string `json:"total_input"`
	TotalOutput string `json:"total_output"`
	Txid        string `json:"txid"`
}
