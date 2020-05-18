package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/lbryio/lbrytv/config"

	"github.com/nsf/jsondiff"
	"github.com/ybbus/jsonrpc"
)

type mockServer struct {
	*httptest.Server
	NextResponse chan<- string
}

func EmptyResponse() string { return "" } // helper method to make it clearer what's happening

func (m *mockServer) QueueResponses(responses ...string) {
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
	next := make(chan string, 1)
	return &mockServer{
		NextResponse: next,
		Server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()
			if requestChan != nil {
				data, _ := ioutil.ReadAll(r.Body)
				requestChan <- &Request{r, w, string(data)}
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

// JsonCompact removes insignificant space characters from a JSON string
// It helps compare JSON strings without worrying about whitespace differences
func JsonCompact(jsonStr string) string {
	dst := &bytes.Buffer{}
	err := json.Compact(dst, []byte(jsonStr))
	if err != nil {
		panic(err)
	}
	return dst.String()
}

// JsonCompare compares two json strings.
func jsonCompare(a, b []byte) (bool, string) {
	opts := jsondiff.DefaultConsoleOptions()
	diff, str := jsondiff.Compare(a, b, &opts)
	return diff == jsondiff.FullMatch, str
}

// assert.Equal for JSON - more accurate comparison, pretty diff
func AssertJsonEqual(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	same, diff := jsonCompare(toBytes(expected), toBytes(actual))
	if same {
		return true
	}

	indent := "\t\t"
	diffIndented := regexp.MustCompile("(?m)^").ReplaceAll([]byte(diff), []byte("\t"+indent))[len(indent)+1:]
	tmpl := "\n\tError:" + indent + "JSON not equal\n\tDiff:" + indent + "%s"
	msg := messageFromMsgAndArgs(msgAndArgs...)
	if len(msg) > 0 {
		t.Errorf(tmpl+"\n\tMessages:"+indent+"%s", diffIndented, msg)
	} else {
		t.Errorf(tmpl, diffIndented)
	}
	return false
}

func toBytes(v interface{}) []byte {
	switch s := v.(type) {
	case string:
		return []byte(s)
	case []byte:
		return s
	default:
		panic(fmt.Sprintf("cannot convert %T to byte slice", v))
	}
}

// copied from assert.Fail()
func messageFromMsgAndArgs(msgAndArgs ...interface{}) string {
	if len(msgAndArgs) == 0 || msgAndArgs == nil {
		return ""
	}
	if len(msgAndArgs) == 1 {
		msg := msgAndArgs[0]
		if msgAsStr, ok := msg.(string); ok {
			return msgAsStr
		}
		return fmt.Sprintf("%+v", msg)
	}
	if len(msgAndArgs) > 1 {
		return fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...)
	}
	return ""
}
