package watchman

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
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
	dbName := randomdata.Alphanumeric(32)
	err = olapdb.Connect(dbCfg["url"], dbName)
	s.cleanup = func() {
		olapdb.MigrateDown(dbName)
	}
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
	okRep := olapdb.PlaybackReportAddRequestFactory.MustCreate().(*client.AddRequestBody)
	rbdTooLargeRep := olapdb.PlaybackReportAddRequestFactory.MustCreate().(*client.AddRequestBody)
	rbdTooLargeRep.RebufDuration = rbdTooLargeRep.Duration + 1

	okBody, err := json.Marshal(okRep)
	s.Require().NoError(err)
	rbdTooLargeBody, err := json.Marshal(rbdTooLargeRep)
	s.Require().NoError(err)

	cases := []struct {
		name, origin  string
		body          []byte
		respCode      int
		respBodyRegex string
	}{
		{"Valid", "https://odysee.com", okBody, http.StatusCreated, "^$"},
		{"ValidLocal", "http://localhost:1337", okBody, http.StatusCreated, "^$"},
		{"Empty", "http://localhost:9090", nil, http.StatusBadRequest, `"message":"missing required payload"`},
		{"RebufDurationTooLarge", "http://localhost:9090", rbdTooLargeBody, http.StatusBadRequest, `"message":"rebufferung duration cannot be larger than duration"`},
	}

	for _, c := range cases {
		s.Run(c.name, func() {
			r, err := http.NewRequest(http.MethodPost, s.ts.URL+reporterclt.AddReporterPath(), bytes.NewBuffer(c.body))
			s.Require().NoError(err)
			cl := &http.Client{}
			r.Header.Add("origin", c.origin)
			r.Header.Add("access-control-request-method", http.MethodPost)

			resp, err := cl.Do(r)
			s.Require().NoError(err)
			b, err := ioutil.ReadAll(resp.Body)
			s.NoError(err)
			s.Equal(c.respCode, resp.StatusCode, string(b))
			s.Regexp(regexp.MustCompile(c.respBodyRegex), string(b))
			s.Equal(c.origin, resp.Header.Get("access-control-allow-origin"))
			s.Equal("GET, POST", resp.Header.Get("access-control-allow-methods"))
			s.Equal("content-type", resp.Header.Get("access-control-allow-headers"))
		})
	}

}

func (s *reporterSuite) TearDownSuite() {
	s.cleanup()
}
