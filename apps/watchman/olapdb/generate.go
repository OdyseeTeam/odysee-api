package olapdb

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/lbryio/lbrytv/apps/watchman/gen/reporter"

	"github.com/Pallinder/go-randomdata"
	"github.com/bluele/factory-go/factory"
)

var playbackReportFactory = factory.NewFactory(
	&reporter.PlaybackReport{},
).Attr("URL", func(args factory.Args) (interface{}, error) {
	return fmt.Sprintf(
		"%v#%v",
		strings.ReplaceAll(randomdata.SillyName(), " ", "-"),
		randomdata.Alphanumeric(32),
	), nil
}).Attr("Position", func(args factory.Args) (interface{}, error) {
	return int32(randomdata.Number(0, 1000000)), nil
}).Attr("Dur", func(args factory.Args) (interface{}, error) {
	// return int32(randomdata.Number(0, 60000)), nil
	return int32(30000), nil
}).Attr("RelPosition", func(args factory.Args) (interface{}, error) {
	return int32(randomdata.Number(0, 100)), nil
}).Attr("RebufCount", func(args factory.Args) (interface{}, error) {
	return int32(randomdata.Number(0, 25)), nil
}).Attr("RebufDuration", func(args factory.Args) (interface{}, error) {
	return int32(randomdata.Number(0, 30000)), nil
}).Attr("Format", func(args factory.Args) (interface{}, error) {
	return randomdata.StringSample("hls", "stb"), nil
}).Attr("Player", func(args factory.Args) (interface{}, error) {
	return randomdata.StringSample("use-p1", "use-p2", "sg-p1", "sg-p2"), nil
}).Attr("UserID", func(args factory.Args) (interface{}, error) {
	return int32(randomdata.Number(100000, 150000)), nil
}).Attr("ClientRate", func(args factory.Args) (interface{}, error) {
	v := int32(randomdata.Number(128000, 3000000))
	return &v, nil
}).Attr("Device", func(args factory.Args) (interface{}, error) {
	return randomdata.StringSample("ios", "adr", "web"), nil
}).Attr("T", func(args factory.Args) (interface{}, error) {
	min := time.Now().Add(-15 * 24 * time.Hour).Unix()
	max := time.Now().Unix()
	delta := max - min

	sec := rand.Int63n(delta) + min
	t := time.Unix(sec, 0).Format(time.RFC1123Z)
	return t, nil
})

func Generate(cnt int) {
	rand.Seed(time.Now().UnixNano())
	tx, _ := conn.Begin()
	stmt, err := prepareWrite(tx)
	if err != nil {
		Log.Fatal(err)
	}
	for t := range timeSeries(cnt, time.Now().Add(-1*24*time.Hour)) {
		r := playbackReportFactory.MustCreate().(*reporter.PlaybackReport)
		// r.T = t.UTC().Format(time.RFC1123Z)
		r.T = t.Format(time.RFC1123)
		fmt.Println(t.UTC().Format(time.RFC1123Z), t.Format(time.RFC1123))
		Write(stmt, r, randomdata.StringSample(randomdata.IpV4Address(), randomdata.IpV6Address()))
		// Log.Infof(">>> %+v", r)
	}
	if err := tx.Commit(); err != nil {
		Log.Fatal(err)
	}

	Log.Infof("sent %v records", cnt)
}

func timeSeries(cnt int, start time.Time) <-chan time.Time {
	s := float64(start.Unix())
	e := time.Now().Unix()
	seconds := float64(e) - s
	avgStep := float64(seconds) / float64(cnt)
	ret := make(chan time.Time)
	go func() {
		for i := 0; i < cnt; i++ {
			ret <- time.Unix(int64(s), 0)
			s += avgStep
		}
		close(ret)
	}()
	return ret
}
