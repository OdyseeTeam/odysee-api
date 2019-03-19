package routes

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/lbryio/lbryweb.go/player"
	"github.com/lbryio/lbryweb.go/proxy"
	log "github.com/sirupsen/logrus"
	"github.com/ybbus/jsonrpc"
)

// Index just serves a blank home page
func Index(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// Proxy takes client request body and feeds it to proxy.ForwardCall
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

	lbrynetResponse, err := proxy.ForwardCall(body)
	if err != nil {
		response, _ := json.Marshal(jsonrpc.RPCResponse{Error: &jsonrpc.RPCError{Message: err.Error()}})
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
	if err != nil {
		if err.Error() == "paid stream" {
			w.WriteHeader(http.StatusPaymentRequired)
		} else if w.Header().Get("Content-Type") == "" {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "%v", err)
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
