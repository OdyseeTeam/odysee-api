package test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"

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

// MockJSONRPCServer creates a JSONRPC server that can be used to test clients
// NOTE: if you want to make sure that you get requests in your requestChan one by one, limit the
// channel to a buffer size of 1. then writes to the chan will block until you read it
func MockJSONRPCServer(requestChan chan *RequestData) (*httptest.Server, func(string)) {
	var mu sync.RWMutex
	// needed to retrieve requests that arrived at httpServer for further investigation
	presetResponse := ""
	setNextResponse := func(s string) {
		mu.Lock()
		defer mu.Unlock()
		presetResponse = s
	}

	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := ioutil.ReadAll(r.Body)
		defer r.Body.Close()
		if requestChan != nil {
			requestChan <- &RequestData{r, string(data)} // store the request for inspection
		}
		mu.RLock()
		defer mu.RUnlock()
		fmt.Fprintf(w, presetResponse) // respond with the preset response
	}))

	return httpServer, setNextResponse
}
