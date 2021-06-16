package tsdb

import (
	"fmt"
	"time"

	"github.com/lbryio/lbrytv/apps/watchman/gen/reporter"
	"github.com/lbryio/lbrytv/apps/watchman/log"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxdb2api "github.com/influxdata/influxdb-client-go/v2/api"
)

var (
	bucket, org string
	client      influxdb2.Client
	wapi        influxdb2api.WriteAPI
	qapi        influxdb2api.QueryAPI

	Log = log.Log.Named("tsdb")
)

func Connect(url, token string) {
	client = influxdb2.NewClientWithOptions(
		url, token,
		influxdb2.DefaultOptions().SetBatchSize(100).SetPrecision(time.Second))
	Log.Infof("configured server %v", url)
}

func ConfigBucket(cfgOrg, cfgBucket string) {
	bucket = cfgBucket
	org = cfgOrg
	Log.Infof("configured org %v, bucket %v", org, bucket)
	wapi = client.WriteAPI(org, bucket)
	qapi = client.QueryAPI(org)
	errCh := wapi.Errors()
	go func() {
		for err := range errCh {
			Log.Errorf("write error: %s\n", err.Error())
		}
	}()
}

func Write(r *reporter.PlaybackReport, addr string) {
	area := getClientArea(addr)
	t, err := time.Parse(time.RFC1123Z, *&r.Ts)
	if err != nil {
		t = time.Now()
	}
	ip := influxdb2.NewPoint("playback",
		map[string]string{
			"url":          r.URL,
			"player":       r.Player,
			"format":       r.Format,
			"rel_position": fmt.Sprintf("%v", r.RelPosition),
			"client":       r.Client,
			"device":       r.Device,
			"area":         area,
		},
		map[string]interface{}{
			"client_rate":  *r.ClientRate,
			"buf_count":    r.RebufCount,
			"buf_duration": r.RebufDuration,
		},
		t)
	wapi.WritePoint(ip)
}

func getClientArea(ip string) string {
	return "eu"
}

func Disconnect() {
	Log.Info("flushing pending writes")
	wapi.Flush()
	client.Close()
	Log.Info("server connection closed")
}
