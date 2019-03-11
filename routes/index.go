package routes

import (
	"encoding/json"
	"html/template"
	"io/ioutil"
	"net/http"

	rice "github.com/GeertJohan/go.rice"
	"github.com/lbryio/lbryweb.go/config"
	"github.com/lbryio/lbryweb.go/proxy"
	log "github.com/sirupsen/logrus"
	"github.com/ybbus/jsonrpc"
)

// Index serves the static home page
func Index(w http.ResponseWriter, req *http.Request) {
	// find a rice.Box
	templateBox, err := rice.FindBox("../assets/templates/")
	if err != nil {
		log.Fatal(err)
	}
	// get file contents as string
	templateString, err := templateBox.String("index.html")
	if err != nil {
		log.Fatal(err)
	}
	// parse and execute the template
	tmplMessage, err := template.New("index").Parse(templateString)
	if err != nil {
		log.Fatal(err)
	}

	w.WriteHeader(http.StatusOK)
	tmplMessage.Execute(w, map[string]string{"Static": config.Settings.GetString("StaticURLPrefix")})
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
