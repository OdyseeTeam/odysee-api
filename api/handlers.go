package api

import (
	"fmt"
	"net/http"

	"github.com/lbryio/lbrytv/app/player"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"

	"github.com/gorilla/mux"
)

var logger = monitor.NewModuleLogger("api")

var Collector = metrics.NewCollector()

// Index serves a blank home page
func Index(w http.ResponseWriter, req *http.Request) {
	http.Redirect(w, req, config.GetProjectURL(), http.StatusSeeOther)
}

func stream(uri string, w http.ResponseWriter, req *http.Request) {
	Collector.MetricsIncrement("player_instances_count", metrics.One)
	Collector.MetricsIncrement("player_streams_total", metrics.One)
	err := player.PlayURI(uri, w, req)
	Collector.MetricsDecrement("player_instances_count", metrics.One)
	// Only output error if player has not pushed anything to the client yet
	if err != nil {
		if err.Error() == "paid stream" {
			w.WriteHeader(http.StatusPaymentRequired)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			monitor.CaptureException(err, map[string]string{"uri": uri})
			w.Write([]byte(err.Error()))
		}
	}
}

// ContentByClaimsURI streams content requested by URI to the browser
func ContentByClaimsURI(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	uri := fmt.Sprintf("%s#%s", vars["uri"], vars["claim"])
	stream(uri, w, req)
}

// ContentByURL streams content requested by URI to the browser
func ContentByURL(w http.ResponseWriter, req *http.Request) {
	stream(req.URL.RawQuery, w, req)
}
