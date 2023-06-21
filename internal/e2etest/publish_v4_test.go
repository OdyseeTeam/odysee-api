package e2etest

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/asynquery"
	"github.com/OdyseeTeam/odysee-api/app/auth"
	"github.com/OdyseeTeam/odysee-api/app/query"
	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/apps/forklift"
	"github.com/OdyseeTeam/odysee-api/apps/uploads"
	"github.com/OdyseeTeam/odysee-api/internal/test"
	"github.com/OdyseeTeam/odysee-api/internal/testdeps"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/configng"
	"github.com/OdyseeTeam/odysee-api/pkg/keybox"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"
	"github.com/OdyseeTeam/odysee-api/pkg/redislocker"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/mux"
	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
	"github.com/stretchr/testify/suite"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"github.com/ybbus/jsonrpc"
)

type publishV4Suite struct {
	suite.Suite

	uploadsHelper  *uploads.TestHelper
	forkliftHelper *forklift.TestHelper
	userHelper     *UserTestHelper
	redisHelper    *testdeps.RedisTestHelper

	s3c               *s3.Client
	uploadServer      *httptest.Server
	uploadsLauncher   *uploads.Launcher
	uploadsRouter     chi.Router
	asynqueryLauncher *asynquery.Launcher
	asynqueryServer   *httptest.Server
	asynqueryRouter   *mux.Router
}

func (s *publishV4Suite) TestPublish() {
	var resp *http.Response
	require := s.Require()
	assert := s.Assert()
	t := s.T()

	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	var fileSize uint64 = 1024 * 1024 * 10
	var chunkSize uint64 = 1024 * 1024 * 2
	var uploadID string

	// Getting the upload token
	resp = (&test.HTTPTest{
		Method: http.MethodPost,
		URL:    s.asynqueryServer.URL + "/api/v1/asynqueries/uploads/",
		Code:   http.StatusOK,
		ReqHeader: map[string]string{
			wallet.AuthorizationHeader: s.userHelper.TokenHeader,
		},
	}).RunHTTP(t)
	defer resp.Body.Close()

	rr := &asynquery.Response{}
	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(err)
	require.NoError(json.Unmarshal(body, rr))
	assert.Empty(rr.Error)
	require.Equal(asynquery.StatusUploadTokenCreated, rr.Status)
	assert.NotEmpty(rr.Payload.(asynquery.UploadTokenCreatedPayload).Token)

	uploadTokenHeader := fmt.Sprintf("Bearer %s", rr.Payload.(asynquery.UploadTokenCreatedPayload).Token)

	uploadFile := s.createRandomFile(fileSize)
	defer uploadFile.Close()

	uploadsURL := s.uploadServer.URL + "/v1/uploads/"

	// Starting the upload.
	resp = (&test.HTTPTest{
		Method: http.MethodPost,
		URL:    uploadsURL,
		Code:   http.StatusCreated,
		ReqHeader: map[string]string{
			uploads.AuthorizationHeader: uploadTokenHeader,
			"Tus-Resumable":             "1.0.0",
			"Upload-Metadata":           fmt.Sprintf("filename %s", base64.StdEncoding.EncodeToString([]byte(uploadFile.Name()))),
			"Upload-Length":             fmt.Sprintf("%d", fileSize),
		},
	}).RunHTTP(t)

	loc, err := url.Parse(resp.Header.Get("Location"))

	require.NoError(err)
	uploadID = filepath.Base(loc.Path)
	tusUploadURL := uploadsURL + uploadID // loc.RequestURI()

	// Uploading the file in chunks
	for i := uint64(0); i < fileSize; i += chunkSize {
		end := i + chunkSize
		if end > fileSize {
			end = fileSize
		}

		chunk := make([]byte, end-i)
		_, err := uploadFile.Read(chunk)
		require.NoError(err)

		(&test.HTTPTest{
			Method: http.MethodPatch,
			URL:    tusUploadURL,
			Code:   http.StatusNoContent,
			ReqHeader: map[string]string{
				uploads.AuthorizationHeader: uploadTokenHeader,
				"Tus-Resumable":             "1.0.0",
				"Upload-Offset":             fmt.Sprintf("%d", i),
				"Content-Type":              "application/offset+octet-stream",
			},
			ReqBody: bytes.NewReader(chunk),
		}).RunHTTP(t)
	}

	// Sending off a JSON-RPC request for stream creation
	streamCreateReq, err := json.Marshal(jsonrpc.NewRequest(query.MethodStreamCreate, map[string]interface{}{
		"name":                  "publish4test",
		"title":                 "Publish v4 test",
		"description":           "",
		"locations":             []string{},
		"bid":                   "0.01000000",
		"languages":             []string{"en"},
		"tags":                  []string{"c:disable-comments"},
		"thumbnail_url":         "https://thumbs.odycdn.com/92399dc6df41af6f7c61def97335dfa5.webp",
		"release_time":          1661882701,
		"blocking":              true,
		"preview":               false,
		"license":               "None",
		"channel_id":            "febc557fcfbe5c1813eb621f7d38a80bc4355085",
		"allow_duplicate_name":  true,
		asynquery.FilePathParam: "https://uploads-v4.api.na-backend.odysee.com/v1/uploads/" + uploadID,
	}))
	require.NoError(err)

	resp = (&test.HTTPTest{
		Method: http.MethodPost,
		URL:    s.asynqueryServer.URL + "/api/v1/asynqueries/",
		Code:   http.StatusCreated,
		ReqHeader: map[string]string{
			wallet.AuthorizationHeader: s.userHelper.TokenHeader,
		},
		ReqBody: bytes.NewReader(streamCreateReq),
	}).RunHTTP(t)

	var query *models.Asynquery
	Wait(s.T(), "successful query settling in the database", 45*time.Second, 1000*time.Millisecond, func() error {
		mods := []qm.QueryMod{
			models.AsynqueryWhere.UploadID.EQ(uploadID),
			models.AsynqueryWhere.UserID.EQ(s.userHelper.UserID()),
			models.AsynqueryWhere.Status.EQ(models.AsynqueryStatusSucceeded),
		}
		query, err = models.Asynqueries(mods...).One(s.userHelper.DB)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrWaitContinue
		} else if err != nil {
			return err
		}
		return nil
	})

	// Checking the status of the query
	resp = (&test.HTTPTest{
		Method: http.MethodGet,
		URL:    s.asynqueryServer.URL + "/api/v1/asynqueries/" + query.ID,
		ReqHeader: map[string]string{
			wallet.AuthorizationHeader: s.userHelper.TokenHeader,
		},
		Code: http.StatusOK,
	}).RunHTTP(t)

	createResponse := StreamCreateResponse{}
	var rpcResponse *jsonrpc.RPCResponse
	decoder := json.NewDecoder(resp.Body)
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	err = decoder.Decode(&rpcResponse)
	require.NoError(err)

	err = ljsonrpc.Decode(rpcResponse.Result, &createResponse)
	require.NoError(err)
	// require.NoError(json.Unmarshal(body, &createResponse))
	assert.Equal("publish4test", createResponse.Outputs[0].Name)
}

func (s *publishV4Suite) SetupSuite() {
	var err error
	require := s.Require()
	t := s.T()

	s.uploadsHelper, err = uploads.NewTestHelper(t)
	require.NoError(err)

	client, err := configng.NewS3Client(s.uploadsHelper.S3Config)
	require.NoError(err)

	s.redisHelper = testdeps.NewRedisTestHelper(t)

	locker, err := redislocker.New(s.redisHelper.Opts)
	require.NoError(err)

	kf, err := keybox.GenerateKeyfob()
	require.NoError(err)

	// Uploads service setup
	s.uploadsLauncher = uploads.NewLauncher(
		uploads.WithFileLocker(locker),
		uploads.WithS3Client(client),
		uploads.WithS3Bucket(s.uploadsHelper.S3Config.Bucket),
		uploads.WithDB(s.uploadsHelper.DB),
		uploads.WithPublicKey(kf.PublicKey()),
		uploads.WithLogger(zapadapter.NewKV(nil)),
		uploads.WithForkliftRequestsConnURL(s.redisHelper.URL),
	)
	s.uploadsRouter, err = s.uploadsLauncher.Build()
	require.NoError(err)
	s.uploadServer = httptest.NewServer(s.uploadsRouter)
	t.Cleanup(s.uploadServer.Close)
	t.Cleanup(s.uploadsLauncher.StartShutdown)

	s.uploadsHelper, err = uploads.NewTestHelper(t)
	require.NoError(err)

	// Forklift service setup
	s.s3c, err = configng.NewS3ClientV2(s.uploadsHelper.S3Config)
	require.NoError(err)

	s.forkliftHelper, err = forklift.NewTestHelper(t)
	if err != nil {
		if errors.Is(err, forklift.ErrMissingEnv) {
			t.Skipf(err.Error())
		} else {
			s.FailNow(err.Error())
		}
	}

	retriever := forklift.NewS3Retriever(t.TempDir(), s.s3c)
	l := forklift.NewLauncher(
		forklift.WithReflectorConfig(s.forkliftHelper.ReflectorConfig),
		forklift.WithBlobPath(s.T().TempDir()),
		forklift.WithRetriever(retriever),
		forklift.WithRequestsConnURL(s.redisHelper.URL),
		forklift.WithResponsesConnURL(s.redisHelper.URL),
		forklift.WithLogger(zapadapter.NewKV(nil)),
		forklift.WithDB(s.uploadsHelper.DB),
	)

	queue, err := l.Build()
	require.NoError(err)

	go queue.ServeUntilShutdown()
	t.Cleanup(queue.Shutdown)

	// User machinery setup
	s.userHelper = &UserTestHelper{}
	require.NoError(s.userHelper.Setup(t))

	// Asynquery routes setup
	s.asynqueryRouter = mux.NewRouter().PathPrefix("/api/v1").Subrouter()
	s.asynqueryLauncher = asynquery.NewLauncher(
		asynquery.WithRequestsConnOpts(s.redisHelper.AsynqOpts),
		asynquery.WithLogger(zapadapter.NewKV(nil)),
		asynquery.WithPrivateKey(kf.PrivateKey()),
		asynquery.WithDB(s.userHelper.DB),
	)
	s.asynqueryRouter.Use(auth.Middleware(s.userHelper.Auther))

	err = s.asynqueryLauncher.InstallRoutes(s.asynqueryRouter)
	require.NoError(err)
	go s.asynqueryLauncher.Start()
	s.asynqueryServer = httptest.NewServer(s.asynqueryRouter)

	t.Cleanup(s.asynqueryLauncher.Shutdown)
}

func TestPublishV4Suite(t *testing.T) {
	suite.Run(t, new(publishV4Suite))
}

func (s *publishV4Suite) createRandomFile(fileSize uint64) *os.File {
	require := s.Require()

	f, err := os.CreateTemp(s.T().TempDir(), "random-file-*.bin")
	require.NoError(err)

	data := make([]byte, fileSize)
	_, err = rand.Read(data)
	require.NoError(err)

	_, err = f.Write(data)
	require.NoError(err)

	f, err = os.Open(f.Name())
	require.NoError(err)
	return f
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
