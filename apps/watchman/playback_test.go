package watchman

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/lbryio/lbrytv/apps/environment"
	"github.com/lbryio/lbrytv/apps/watchman/db"
	"github.com/lbryio/lbrytv/apps/watchman/factories"
	reporterclt "github.com/lbryio/lbrytv/apps/watchman/gen/http/reporter/client"
	reportersvr "github.com/lbryio/lbrytv/apps/watchman/gen/http/reporter/server"
	"github.com/lbryio/lbrytv/apps/watchman/gen/reporter"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/storage"

	"github.com/stretchr/testify/suite"
	goahttp "goa.design/goa/v3/http"
)

type playbackSuite struct {
	suite.Suite
	ts      *httptest.Server
	cleanup func()
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(playbackSuite))
}

func (s *playbackSuite) SetupSuite() {
	e := environment.ForWatchman()
	config := e.Get("config").(*config.ConfigWrapper)
	dbConfig := config.GetDatabase()
	params := storage.ConnParams{
		Connection: dbConfig.Connection,
		DBName:     dbConfig.DBName,
		Options:    dbConfig.Options,
	}

	dbConn, connCleanup := storage.CreateTestConn(params)
	dbConn.SetDefaultConnection()

	s.cleanup = connCleanup

	logger := log.New(os.Stderr, "[watchman] ", log.Ltime)
	reporterSvc := NewReporter(dbConn.DB.DB, logger)
	reporterEndpoints := reporter.NewEndpoints(reporterSvc)

	var (
		dec = goahttp.RequestDecoder
		enc = goahttp.ResponseEncoder
	)
	mux := goahttp.NewMuxer()
	reporterServer := reportersvr.New(reporterEndpoints, mux, dec, enc, nil, nil)
	reportersvr.Mount(mux, reporterServer)
	s.ts = httptest.NewServer(mux)
}

func (s *playbackSuite) TestAdd() {
	r := s.newAddRequest(factories.GeneratePlaybackReport())
	doer := &http.Client{}
	resp, err := doer.Do(r)
	s.Require().NoError(err)
	b, err := ioutil.ReadAll(resp.Body)
	s.NoError(err)
	s.Equal(http.StatusCreated, resp.StatusCode, string(b))
}

func (s *playbackSuite) newAddRequest(p db.CreatePlaybackReportParams) *http.Request {
	qBody, err := json.Marshal(p)
	fmt.Println(string(qBody))
	s.Require().NoError(err)
	r, err := http.NewRequest("POST", s.ts.URL+reporterclt.AddReporterPath(), bytes.NewBuffer(qBody))
	s.Require().NoError(err)
	return r
}

func (s *playbackSuite) TearDownSuite() {
	s.cleanup()
}
