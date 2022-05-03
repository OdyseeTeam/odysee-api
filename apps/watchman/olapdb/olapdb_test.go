package olapdb

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/lbryio/lbrytv/apps/watchman/config"
	"github.com/lbryio/lbrytv/apps/watchman/gen/reporter"
	"github.com/stretchr/testify/suite"
)

type BaseOlapdbSuite struct {
	suite.Suite
	cleanup func()
}

type olapdbSuite struct {
	BaseOlapdbSuite
}

func TestOlapdbSuite(t *testing.T) {
	suite.Run(t, new(olapdbSuite))
}

func (s *BaseOlapdbSuite) SetupSuite() {
	cfg, err := config.Read()
	s.Require().NoError(err)

	dbCfg := cfg.GetStringMapString("clickhouse")
	dbName := randomdata.Alphanumeric(32)
	err = Connect(dbCfg["url"], dbName)
	s.cleanup = func() {
		MigrateDown(dbName)

	}
	s.Require().NoError(err)

	p, _ := filepath.Abs(filepath.Join("./testdata", "GeoIP2-City-Test.mmdb"))
	err = OpenGeoDB(p)
	s.Require().NoError(err)
}

func (s *olapdbSuite) TestWriteOne() {
	r := PlaybackReportFactory.MustCreate().(*reporter.PlaybackReport)
	ts := time.Now().Format(time.RFC1123Z)
	err := WriteOne(r, randomdata.StringSample(randomdata.IpV4Address(), randomdata.IpV6Address()), ts)
	s.Require().NoError(err)

	var (
		url      string
		duration int32
	)
	rows, err := conn.Query(fmt.Sprintf("select URL, Duration from %s.playback where URL = ?", database), r.URL)
	s.Require().NoError(err)
	defer rows.Close()
	rows.Next()
	err = rows.Scan(&url, &duration)
	s.Require().NoError(err)
	s.Equal(r.Duration, duration)
}

func (s *olapdbSuite) TestWriteGarbled() {
	r := PlaybackReportFactory.MustCreate().(*reporter.PlaybackReport)
	cleanPlayer := r.Player
	r.Player += ", some-nonsense-at-the-end-abcbcbdagadwsedaddff"
	ts := time.Now().Format(time.RFC1123Z)
	err := WriteOne(r, randomdata.StringSample(randomdata.IpV4Address(), randomdata.IpV6Address()), ts)
	s.Require().NoError(err)

	var (
		url      string
		duration int32
		player   string
	)
	rows, err := conn.Query(fmt.Sprintf("select URL, Duration, Player from %s.playback where URL = ?", database), r.URL)
	s.Require().NoError(err)
	defer rows.Close()
	rows.Next()
	err = rows.Scan(&url, &duration, &player)
	s.Require().NoError(err)
	s.Equal(r.Duration, duration)
	s.Equal(r.URL, url)
	s.Equal(r.URL, url)
	s.Equal(cleanPlayer, strings.Trim(player, "\x00"))
}
