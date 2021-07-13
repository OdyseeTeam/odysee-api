package olapdb

import (
	"database/sql"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/lbryio/lbrytv/apps/watchman/gen/http/reporter/client"
	"github.com/lbryio/lbrytv/apps/watchman/gen/reporter"
	"github.com/lbryio/lbrytv/apps/watchman/log"

	"github.com/Pallinder/go-randomdata"
	"github.com/bluele/factory-go/factory"
)

var PlaybackReportFactory = EnrichPlaybackReportFactory(factory.NewFactory(&reporter.PlaybackReport{}))
var PlaybackReportAddRequestFactory = EnrichPlaybackReportFactory(factory.NewFactory(&client.AddRequestBody{}))

func EnrichPlaybackReportFactory(f *factory.Factory) *factory.Factory {
	return f.Attr("URL", func(args factory.Args) (interface{}, error) {
		return fmt.Sprintf(
			"%v#%v",
			strings.ReplaceAll(randomdata.SillyName(), " ", "-"),
			randomdata.Alphanumeric(32),
		), nil
	}).Attr("Position", func(args factory.Args) (interface{}, error) {
		return int32(randomdata.Number(0, 1000000)), nil
	}).Attr("Duration", func(args factory.Args) (interface{}, error) {
		return int32(30000), nil
	}).Attr("RelPosition", func(args factory.Args) (interface{}, error) {
		return int32(randomdata.Number(0, 100)), nil
	}).Attr("RebufCount", func(args factory.Args) (interface{}, error) {
		return int32(randomdata.Number(0, 25)), nil
	}).Attr("RebufDuration", func(args factory.Args) (interface{}, error) {
		return int32(randomdata.Number(0, 30000)), nil
	}).Attr("Protocol", func(args factory.Args) (interface{}, error) {
		return randomdata.StringSample("hls", "stb"), nil
	}).Attr("Player", func(args factory.Args) (interface{}, error) {
		return randomdata.StringSample("use-p1", "use-p2", "sg-p1", "sg-p2"), nil
	}).Attr("UserID", func(args factory.Args) (interface{}, error) {
		return int32(randomdata.Number(100000, 150000)), nil
	}).Attr("Bandwidth", func(args factory.Args) (interface{}, error) {
		v := int32(randomdata.Number(128000, 3000000))
		return &v, nil
	}).Attr("Device", func(args factory.Args) (interface{}, error) {
		return randomdata.StringSample("ios", "adr", "web"), nil
	})
}

func Generate(number int, days int) {
	var (
		stmt *sql.Stmt
		tx   *sql.Tx
	)
	rand.Seed(time.Now().UnixNano())

	l := log.Log.Named("clickhouse.generate")
	counter := 1

	tx, _ = conn.Begin()
	stmt, err := prepareWrite(tx)
	if err != nil {
		l.Fatal(err)
	}

	for t := range timeSeries(number, time.Now().Add(time.Duration(-days)*24*time.Hour)) {
		r := PlaybackReportFactory.MustCreate().(*reporter.PlaybackReport)
		ts := t.Format(time.RFC1123Z)
		err := Write(stmt, r, randomdata.StringSample(randomdata.IpV4Address(), randomdata.IpV6Address()), ts)
		if err != nil {
			l.Fatal(err)
		}
		if counter%100 == 0 {
			if err := tx.Commit(); err != nil {
				l.Warn(err)
			} else {
				l.Infof("sent %v records", counter)
			}
			tx, _ = conn.Begin()
			stmt, err = prepareWrite(tx)
			if err != nil {
				l.Fatal(err)
			}
		}
		counter++
	}
	if err := tx.Commit(); err != nil {
		l.Warn(err)
	} else {
		l.Infof("total of %v records sent", counter)
	}

}

func timeSeries(number int, start time.Time) <-chan time.Time {
	l := log.Log.Named("clickhouse.generate")

	startSec := float64(start.Unix())
	endSec := float64(time.Now().Unix())

	totalSec := float64(endSec) - startSec
	avgStep := float64(totalSec) / float64(number)
	l.Infof("generating %v time entries from approx. %v to %v, interval=%vs", number, start.Format(time.RFC1123Z), time.Now().Format(time.RFC1123Z), avgStep)
	ret := make(chan time.Time)
	go func() {
		for i := 0; i < number; i++ {
			sec, dec := math.Modf(endSec)
			ret <- time.Unix(int64(sec), int64(dec*(1e9)))
			endSec -= float64(rand.Float64() * avgStep * 2)
		}
		close(ret)
	}()
	return ret
}
