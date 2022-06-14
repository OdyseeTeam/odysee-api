package metrics

import (
	"encoding/json"
	"net/http"

	"github.com/OdyseeTeam/odysee-api/internal/monitor"
	"github.com/OdyseeTeam/odysee-api/internal/responses"

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
	case "time_to_start":
		player := req.FormValue("player")
		if len(player) > 64 {
			code = http.StatusBadRequest
			resp["error"] = "invalid player value"
		}
		UITimeToStart.WithLabelValues(player).Observe(cast.ToFloat64(req.FormValue("value")))
	default:
		code = http.StatusBadRequest
		resp["error"] = "invalid metric name"
	}

	w.WriteHeader(code)
	respByte, _ := json.Marshal(&resp)
	w.Write(respByte)
}
