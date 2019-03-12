package routes

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/lbryio/lbryweb.go/player"
)

// ContentByClaimsURI streams content requested by URI to the browser
func ContentByClaimsURI(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	uri := fmt.Sprintf("%s#%s", vars["uri"], vars["claim"])
	err := player.PlayURI(uri, req.Header.Get("Range"), w)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "%v", err)
	}
}

// ContentByURL streams content requested by URI to the browser
func ContentByURL(w http.ResponseWriter, req *http.Request) {
	err := player.PlayURI(req.URL.RawQuery, req.Header.Get("Range"), w)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "%v", err)
	}
}
