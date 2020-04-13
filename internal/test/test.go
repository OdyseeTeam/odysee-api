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

type Request struct {
	R    *http.Request
	Body string
}

// MockHTTPServer creates an http server that can be used to test clients
// NOTE: if you want to make sure that you get requests in your requestChan one by one, limit the
// channel to a buffer size of 1. then writes to the chan will block until you read it. see
// ReqChan() for how to do this
func MockHTTPServer(requestChan chan *Request) *MockServer {
	next := make(chan string, 1)
	return &MockServer{
		NextResponse: next,
		Server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()
			if requestChan != nil {
				data, _ := ioutil.ReadAll(r.Body)
				requestChan <- &Request{r, string(data)}
			}
			fmt.Fprintf(w, <-next)
		})),
	}
}

// ReqChan makes a channel for reading received requests one by one.
// Use it in conjunction with MockHTTPServer
func ReqChan() chan *Request {
	return make(chan *Request, 1)
}

// ReqToStr is a convenience method
func ReqToStr(t *testing.T, req jsonrpc.RPCRequest) string {
	r, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	return string(r)
}

// StrToReq is a convenience method
func StrToReq(t *testing.T, req string) jsonrpc.RPCRequest {
	var r jsonrpc.RPCRequest
	err := json.Unmarshal([]byte(req), &r)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// ResToStr is a convenience method
func ResToStr(t *testing.T, res jsonrpc.RPCResponse) string {
	r, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	return string(r)
}
