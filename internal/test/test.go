package test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/config"
)

func SDKRouter() *sdkrouter.Router {
	return sdkrouter.New(config.GetLbrynetServers())
}

type RequestData struct {
	Request *http.Request
	Body    string
}

func MockJSONRPCServer() (*httptest.Server, chan *RequestData, func(string)) {
	// needed to retrieve requests that arrived at httpServer for further investigation
	requestChan := make(chan *RequestData, 1)

	responseBody := ""
	setNextResponse := func(s string) { responseBody = s }

	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := ioutil.ReadAll(r.Body)
		defer r.Body.Close()
		requestChan <- &RequestData{r, string(data)} // store the request
		fmt.Fprintf(w, responseBody)                 // write the preset response
	}))

	return httpServer, requestChan, setNextResponse
}
