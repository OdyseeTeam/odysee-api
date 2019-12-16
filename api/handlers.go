package api

import (
	"net/http"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/monitor"
)

var logger = monitor.NewModuleLogger("api")

// Index serves a blank home page
func Index(w http.ResponseWriter, req *http.Request) {
	http.Redirect(w, req, config.GetProjectURL(), http.StatusSeeOther)
}
