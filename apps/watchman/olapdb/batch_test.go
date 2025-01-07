package olapdb

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/OdyseeTeam/odysee-api/apps/watchman/gen/reporter"
	"github.com/OdyseeTeam/odysee-api/apps/watchman/log"
	"github.com/Pallinder/go-randomdata"
	"github.com/stretchr/testify/suite"
)

type batchWriterSuite struct {
	BaseOlapdbSuite
}

func TestBatchWriterSuite(t *testing.T) {
	log.Configure(log.LevelInfo, log.EncodingConsole)
	suite.Run(t, new(batchWriterSuite))
}

func (s *batchWriterSuite) TestBatch() {
	days := 14 * 24 * time.Hour
	number := 1000
	reports := []*reporter.PlaybackReport{}
	bw := NewBatchWriter(100*time.Millisecond, 16)
	go bw.Start()

	for t := range timeSeries(number, time.Now().Add(-days)) {
		r := PlaybackReportFactory.MustCreate().(*reporter.PlaybackReport)
		ts := t.Format(time.RFC1123Z)
		err := bw.Write(r, randomdata.StringSample(randomdata.IpV4Address(), randomdata.IpV6Address()), ts)
		s.Require().NoError(err)
		reports = append(reports, r)
	}

	time.Sleep(3 * time.Second)
	bw.Stop()

	rows, err := conn.Query(fmt.Sprintf("SELECT URL, Duration from %s.playback ORDER BY Timestamp DESC", database))
	s.Require().NoError(err)
	defer rows.Close()
	type duration struct {
		URL      string
		Duration int32
	}
	retDurations := []duration{}
	for i, r := range reports {
		n := rows.Next()
		s.Require().True(n, "only %v rows in db, expected %v", i, len(reports))
		d := duration{}
		err = rows.Scan(&d.URL, &d.Duration)
		s.Require().NoError(err)
		retDurations = append(retDurations, d)
		s.Equal(r.URL, d.URL)
		s.Equal(r.Duration, d.Duration)
	}
	s.False(rows.Next())

	sort.Slice(retDurations, func(i, j int) bool {
		return retDurations[i].Duration < retDurations[j].Duration
	})
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].Duration < reports[j].Duration
	})

	s.Equal(len(reports), len(retDurations))
	for i := range reports {
		s.Equal(reports[i].URL, retDurations[i].URL)
		s.Equal(reports[i].Duration, retDurations[i].Duration)
	}
}
