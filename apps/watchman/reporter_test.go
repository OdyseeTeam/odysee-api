package watchman

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/lbryio/lbrytv/apps/watchman/config"
	"github.com/lbryio/lbrytv/apps/watchman/gen/http/reporter/client"
	reporterclt "github.com/lbryio/lbrytv/apps/watchman/gen/http/reporter/client"
	reportersvr "github.com/lbryio/lbrytv/apps/watchman/gen/http/reporter/server"
	"github.com/lbryio/lbrytv/apps/watchman/gen/reporter"
	"github.com/lbryio/lbrytv/apps/watchman/log"
	"github.com/lbryio/lbrytv/apps/watchman/olapdb"

	"github.com/Pallinder/go-randomdata"
	"github.com/stretchr/testify/suite"
	goahttp "goa.design/goa/v3/http"
)

type reporterSuite struct {
	suite.Suite
	ts      *httptest.Server
	cleanup func()
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(reporterSuite))
}

func (s *reporterSuite) SetupSuite() {
	cfg, err := config.Read()
	if err != nil {
		log.Log.Fatal(err)
	}

	log.Configure(log.LevelDebug, log.EncodingConsole)

	dbCfg := cfg.GetStringMapString("clickhouse")
	err = olapdb.Connect(dbCfg["url"], randomdata.Alphanumeric(32))
	s.Require().NoError(err)

	p, _ := filepath.Abs(filepath.Join("./olapdb/testdata", "GeoIP2-City-Test.mmdb"))
	err = olapdb.OpenGeoDB(p)
	s.Require().NoError(err)

	reporterSvc := NewReporter(nil, log.Log)
	reporterEndpoints := reporter.NewEndpoints(reporterSvc)

	var (
		dec = goahttp.RequestDecoder
		enc = goahttp.ResponseEncoder
	)
	mux := goahttp.NewMuxer()
	reporterServer := reportersvr.New(reporterEndpoints, mux, dec, enc, nil, nil, nil)
	reporterServer.Use(RemoteAddressMiddleware())
	reportersvr.Mount(mux, reporterServer)
	s.ts = httptest.NewServer(mux)
}

func (s *reporterSuite) TestAdd() {
	rep := olapdb.PlaybackReportAddRequestFactory.MustCreate().(*client.AddRequestBody)
	// rep.T = t.UTC().Format(time.RFC1123Z)
	// Write(stmt, r, randomdata.StringSample(randomdata.IpV4Address(), randomdata.IpV6Address()))

	qBody, err := json.Marshal(rep)
	s.Require().NoError(err)
	r, err := http.NewRequest(http.MethodPost, s.ts.URL+reporterclt.AddReporterPath(), bytes.NewBuffer(qBody))
	s.Require().NoError(err)

	c := &http.Client{}
	resp, err := c.Do(r)
	s.Require().NoError(err)
	b, err := ioutil.ReadAll(resp.Body)
	s.NoError(err)
	s.Equal(http.StatusCreated, resp.StatusCode, string(b))
	s.True(false)
}

// func (s *reporterSuite) newAddRequest(p db.CreatePlaybackReportParams) *http.Request {
// 	qBody, err := json.Marshal(p)
// 	fmt.Println(string(qBody))
// 	s.Require().NoError(err)
// 	r, err := http.NewRequest("POST", s.ts.URL+reporterclt.AddReporterPath(), bytes.NewBuffer(qBody))
// 	s.Require().NoError(err)
// 	return r
// }

func (s *reporterSuite) TearDownSuite() {
	// s.cleanup()
}
