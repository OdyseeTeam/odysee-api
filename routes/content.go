package routes

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/lbryio/lbryweb.go/player"
)

// ContentByURI streams content requested by URI to the browser
func ContentByURI(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	err := player.PlayURI(vars["uri"], req.Header.Get("Range"), w)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "%v", err)
	}
}
