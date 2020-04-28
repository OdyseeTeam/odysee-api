package metrics

import (
	"encoding/json"
	"net/http"

	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/internal/responses"

	"github.com/spf13/cast"
)

var Logger = monitor.NewModuleLogger("metrics")

func TrackUIMetric(w http.ResponseWriter, req *http.Request) {
	responses.AddJSONContentType(w)
	resp := make(map[string]string)
	code := http.StatusOK

	metricName := req.FormValue("name")
	resp["name"] = metricName

	switch metricName {
	case "buffer":
		UIBufferCount.Inc()
	case "time_to_start":
		t := cast.ToFloat64(req.FormValue("value"))
		if t == 0 {
			Logger.Log().Errorf("Time to start cannot be 0")
			code = http.StatusBadRequest
			resp["error"] = "Time to start cannot be 0"
		} else {
			UITimeToStart.Observe(t)
		}
	default:
		Logger.Log().Errorf("invalid UI metric name: %s", metricName)
		code = http.StatusBadRequest
		resp["error"] = "Invalid metric name"
	}

	w.WriteHeader(code)
	respByte, _ := json.Marshal(&resp)
	w.Write(respByte)
}
