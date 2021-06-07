package tsdb

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
}).Attr("RelPosition", func(args factory.Args) (interface{}, error) {
	return int32(randomdata.Number(0, 100)), nil
}).Attr("BufCount", func(args factory.Args) (interface{}, error) {
	return int32(randomdata.Number(0, 25)), nil
}).Attr("BufDuration", func(args factory.Args) (interface{}, error) {
	return int32(randomdata.Number(0, 10000)), nil
}).Attr("Format", func(args factory.Args) (interface{}, error) {
	return randomdata.StringSample("hls", "std"), nil
}).Attr("Player", func(args factory.Args) (interface{}, error) {
	return randomdata.StringSample("use-p1", "use-p2", "sg-p1", "sg-p2"), nil
}).Attr("Client", func(args factory.Args) (interface{}, error) {
	// r := args.Instance().(*reporter.PlaybackReport)
	return randomdata.Alphanumeric(32), nil
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
	return &t, nil
})

func Generate(cnt int) {
	rand.Seed(time.Now().UnixNano())
	Write(
		playbackReportFactory.MustCreate().(*reporter.PlaybackReport),
		randomdata.StringSample(randomdata.IpV4Address(), randomdata.IpV6Address()),
	)
}
