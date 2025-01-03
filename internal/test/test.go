package test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"

	"github.com/ybbus/jsonrpc/v2"
)

type mockServer struct {
	*httptest.Server
	NextResponse chan<- any
}

func EmptyResponse() string { return "" } // helper method to make it clearer what's happening

func (m *mockServer) QueueResponses(responses ...any) {
	go func() {
		for _, r := range responses {
			m.NextResponse <- r
		}
	}()
}

type Request struct {
	R    *http.Request
	W    http.ResponseWriter
	Body string
}

// MockHTTPServer creates an http server that can be used to test clients
// NOTE: if you want to make sure that you get requests in your requestChan one by one, limit the
// channel to a buffer size of 1. then writes to the chan will block until you read it. see
// ReqChan() for how to do this
func MockHTTPServer(requestChan chan *Request) *mockServer {
	next := make(chan any, 1)
	return &mockServer{
		NextResponse: next,
		Server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()
			if requestChan != nil {
				data, _ := io.ReadAll(r.Body)
				requestChan <- &Request{r, w, string(data)}
			}
			nextResponse := <-next
			switch v := nextResponse.(type) {
			case string:
				fmt.Fprint(w, v)
			case int:
				w.WriteHeader(v)
			case func(http.ResponseWriter, *http.Request):
				v(w, r)
			default:
				fmt.Printf("test server: unknown type %T\n", v)
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(fmt.Sprintf("test server: unknown type %T\n", v)))
			}
		})),
	}
}

// ReqChan makes a channel for reading received requests one by one.
// Use it in conjunction with MockHTTPServer
func ReqChan() chan *Request {
	return make(chan *Request, 999)
}

// ReqToStr stringifies a supplied RPCRequest
func ReqToStr(t *testing.T, req *jsonrpc.RPCRequest) string {
	t.Helper()
	r, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	return string(r)
}

// StrToReq creates an RPCRequest from a supplied string
func StrToReq(t *testing.T, req string) *jsonrpc.RPCRequest {
	t.Helper()
	var r *jsonrpc.RPCRequest
	err := json.Unmarshal([]byte(req), &r)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// ResToStr stringifies a supplied RPCResponse
func ResToStr(t *testing.T, res *jsonrpc.RPCResponse) string {
	t.Helper()
	r, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	return string(r)
}

// StrToRes creates an RPCResponse from a supplied string
func StrToRes(t *testing.T, res string) *jsonrpc.RPCResponse {
	t.Helper()
	var r *jsonrpc.RPCResponse
	err := json.Unmarshal([]byte(res), &r)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func RandServerAddress(t *testing.T) string {
	for _, addr := range config.GetLbrynetServers() {
		return addr
	}
	t.Fatal("no lbrynet servers configured")
	return ""
}

func NetworkErrorResponse(w http.ResponseWriter, r *http.Request) {
	hj, _ := w.(http.Hijacker)
	conn, _, _ := hj.Hijack()
	conn.Close()
}
