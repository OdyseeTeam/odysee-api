package test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ybbus/jsonrpc"
)

type MockServer struct {
	*httptest.Server
	NextResponse chan<- string
}

func (m *MockServer) NoMoreResponses() {
	close(m.NextResponse)
}

type RequestData struct {
	R    *http.Request
	Body string
}

// MockHTTPServer creates an http server that can be used to test clients
// NOTE: if you want to make sure that you get requests in your requestChan one by one, limit the
// channel to a buffer size of 1. then writes to the chan will block until you read it
func MockHTTPServer(requestChan chan *RequestData) *MockServer {
	next := make(chan string, 1)
	return &MockServer{
		NextResponse: next,
		Server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			data, _ := ioutil.ReadAll(r.Body)
			defer r.Body.Close()
			if requestChan != nil {
				requestChan <- &RequestData{r, string(data)} // store the request for inspection
			}
			fmt.Fprintf(w, <-next)
		})),
	}
}

func ReqToStr(t *testing.T, req jsonrpc.RPCRequest) string {
	r, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	return string(r)
}

func StrToReq(t *testing.T, req string) jsonrpc.RPCRequest {
	var r jsonrpc.RPCRequest
	err := json.Unmarshal([]byte(req), &r)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func ResToStr(t *testing.T, res jsonrpc.RPCResponse) string {
	r, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	return string(r)
}
