package routes

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ljsonrpc "github.com/lbryio/lbry.go/extras/jsonrpc"
	"github.com/stretchr/testify/assert"
	"github.com/ybbus/jsonrpc"
)

func TestProxyNilQuery(t *testing.T) {
	request, _ := http.NewRequest("POST", "/api/proxy", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(Proxy).ServeHTTP(rr, request)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "empty request body", rr.Body.String())
}

func TestProxyNonsenseQuery(t *testing.T) {
	var parsedResponse jsonrpc.RPCResponse

	request, _ := http.NewRequest("POST", "/api/proxy", bytes.NewBuffer([]byte("yo")))
	rr := httptest.NewRecorder()
	http.HandlerFunc(Proxy).ServeHTTP(rr, request)
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	err := json.Unmarshal(rr.Body.Bytes(), &parsedResponse)
	if err != nil {
		panic(err)
	}
	assert.True(t, strings.HasPrefix(parsedResponse.Error.Message, "client json parse error: invalid character 'y'"))
}

func TestProxy(t *testing.T) {
	var query *jsonrpc.RPCRequest
	var queryBody []byte
	var parsedResponse jsonrpc.RPCResponse
	resolveResponse := make(ljsonrpc.ResolveResponse)

	query = jsonrpc.NewRequest("resolve", map[string]string{"urls": "what"})
	queryBody, _ = json.Marshal(query)
	request, _ := http.NewRequest("POST", "/api/proxy", bytes.NewBuffer(queryBody))
	rr := httptest.NewRecorder()

	http.HandlerFunc(Proxy).ServeHTTP(rr, request)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json; charset=utf-8", rr.HeaderMap["Content-Type"][0])
	err := json.Unmarshal(rr.Body.Bytes(), &parsedResponse)
	if err != nil {
		panic(err)
	}
	ljsonrpc.Decode(parsedResponse.Result, &resolveResponse)
	assert.Equal(t, "what", resolveResponse["what"].Claim.Name)
}

func TestContentByURL_NoPayment(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://localhost:40080/content/url", nil)
	req.URL.RawQuery = "pra-onde-vamos-em-2018-seguran-a-online#3a508cce1fda3b7c1a2502cb4323141d40a2cf0b"
	req.Header.Add("Range", "bytes=0-1023")
	rr := httptest.NewRecorder()
	http.HandlerFunc(ContentByURL).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusPaymentRequired, rr.Code)
	_, err := rr.Body.ReadByte()
	assert.Equal(t, io.EOF, err)
}
