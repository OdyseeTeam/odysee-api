package routes

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/lbryio/lbrytv/db"
	"github.com/lbryio/lbrytv/monitor"
	"github.com/lbryio/lbrytv/player"
	"github.com/lbryio/lbrytv/proxy"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

var logger = monitor.NewModuleLogger("db")

// Index just serves a blank home page
func Index(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// Proxy takes client request body and feeds it to the proxy module
func Proxy(w http.ResponseWriter, req *http.Request) {
	if req.Body == nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("empty request body"))
		return
	}
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Panicf("error: %v", err.Error())
	}

	ur, err := proxy.UnmarshalRequest(body)
	if err != nil {
		response, _ := json.Marshal(proxy.NewErrorResponse(err.Error(), proxy.ErrProxy))
		w.WriteHeader(http.StatusBadRequest)
		w.Write(response)
		return
	}

	accountID, err := db.GetAccountIDFromRequest(req)
	if err != nil {
		response, _ := json.Marshal(proxy.NewErrorResponse(err.Error(), proxy.ErrProxyAuthFailed))
		w.WriteHeader(http.StatusForbidden)
		w.Write(response)
		return
	}

	lbrynetResponse, err := proxy.Proxy(ur, accountID)
	if err != nil {
		logger.LogF(monitor.F{"query": ur, "error": err}).Error("proxy errored")
		response, _ := json.Marshal(proxy.NewErrorResponse(err.Error(), proxy.ErrProxy))
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write(response)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(lbrynetResponse)
}

func stream(uri string, w http.ResponseWriter, req *http.Request) {
	err := player.PlayURI(uri, w, req)
	// Only output error if player has not pushed anything to the client yet
	if err != nil && err.Error() == "paid stream" {
		w.WriteHeader(http.StatusPaymentRequired)
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
