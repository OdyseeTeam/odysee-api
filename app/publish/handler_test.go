package publish

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"

	"github.com/lbryio/lbrytv/app/auth"
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/internal/errors"
	"github.com/lbryio/lbrytv/internal/test"
	"github.com/lbryio/lbrytv/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

type DummyPublisher struct {
	called   bool
	filePath string
	walletID string
	rawQuery string
}

func TestUploadHandler(t *testing.T) {
	r := CreatePublishRequest(t, []byte("test file"))
	r.Header.Set(wallet.TokenHeader, "uPldrToken")

	publisher := &DummyPublisher{}

	reqChan := test.ReqChan()
	ts := test.MockHTTPServer(reqChan)
	go func() {
		req := <-reqChan
		publisher.called = true
		rpcReq := test.StrToReq(t, req.Body)
		params, ok := rpcReq.Params.(map[string]interface{})
		require.True(t, ok)
		publisher.filePath = params["file_path"].(string)
		publisher.walletID = params["wallet_id"].(string)
		publisher.rawQuery = req.Body
		ts.NextResponse <- expectedStreamCreateResponse
	}()

	handler := &Handler{UploadPath: os.TempDir()}

	provider := func(token, ip string) auth.Result {
		if token == "uPldrToken" {
			res := auth.NewResult(&models.User{ID: 20404}, nil)
			res.SDKAddress = ts.URL
			return res
		}
		return auth.NewResult(nil, errors.Base("error"))
	}

	rr := httptest.NewRecorder()
	auth.Middleware(provider)(http.HandlerFunc(handler.Handle)).ServeHTTP(rr, r)
	response := rr.Result()
	respBody, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, response.StatusCode)
	test.AssertJsonEqual(t, expectedStreamCreateResponse, respBody)

	require.True(t, publisher.called)
	expectedPath := path.Join(os.TempDir(), "20404", ".*_lbry_auto_test_file")
	assert.Regexp(t, expectedPath, publisher.filePath)
	assert.Equal(t, sdkrouter.WalletID(20404), publisher.walletID)
	expectedReq := fmt.Sprintf(expectedStreamCreateRequest, sdkrouter.WalletID(20404), publisher.filePath)
	test.AssertJsonEqual(t, expectedReq, publisher.rawQuery)

	_, err = os.Stat(publisher.filePath)
	assert.True(t, os.IsNotExist(err))
}

func TestHandler_NoAuthMiddleware(t *testing.T) {
	r, err := http.NewRequest("POST", "/api/v1/proxy", &bytes.Buffer{})
	require.NoError(t, err)
	r.Header.Set(wallet.TokenHeader, "uPldrToken")

	handler := &Handler{UploadPath: os.TempDir()}

	rr := httptest.NewRecorder()
	assert.Panics(t, func() {
		handler.Handle(rr, r)
	})
}

func TestHandler_NoSDKAddress(t *testing.T) {
	r := CreatePublishRequest(t, []byte("test file"))
	r.Header.Set(wallet.TokenHeader, "x")
	rr := httptest.NewRecorder()

	handler := &Handler{UploadPath: os.TempDir()}
	provider := func(token, ip string) auth.Result {
		return auth.NewResult(&models.User{ID: 20404}, nil)
	}

	auth.Middleware(provider)(http.HandlerFunc(handler.Handle)).ServeHTTP(rr, r)
	response := rr.Result()
	respBody, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Contains(t, string(respBody), "user does not have sdk address assigned")
}

func TestHandler_AuthRequired(t *testing.T) {
	r := CreatePublishRequest(t, []byte("test file"))

	publisher := &DummyPublisher{}
	handler := &Handler{UploadPath: os.TempDir()}

	provider := func(token, ip string) auth.Result {
		if token == "uPldrToken" {
			return auth.NewResult(&models.User{ID: 20404}, nil)
		}
		return auth.NewResult(nil, errors.Base("error"))
	}

	rr := httptest.NewRecorder()
	auth.Middleware(provider)(http.HandlerFunc(handler.Handle)).ServeHTTP(rr, r)
	response := rr.Result()

	assert.Equal(t, http.StatusOK, response.StatusCode)
	var rpcResponse jsonrpc.RPCResponse
	err := json.Unmarshal(rr.Body.Bytes(), &rpcResponse)
	require.NoError(t, err)
	assert.Equal(t, "authentication required", rpcResponse.Error.Message)
	require.False(t, publisher.called)
}

func TestUploadHandlerSystemError(t *testing.T) {
	// Creating POST data manually here because we need to avoid writer.Close()
	reader := bytes.NewReader([]byte("test file"))
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	fileBody, err := writer.CreateFormFile(fileFieldName, "lbry_auto_test_file")
	require.NoError(t, err)
	_, err = io.Copy(fileBody, reader)
	require.NoError(t, err)

	jsonPayload, err := writer.CreateFormField(jsonRPCFieldName)
	require.NoError(t, err)
	jsonPayload.Write([]byte(fmt.Sprintf(expectedStreamCreateRequest, sdkrouter.WalletID(20404), "arst")))

	// <--- Not calling writer.Close() here to create an unexpected EOF

	req, err := http.NewRequest("POST", "/", bytes.NewReader(body.Bytes()))
	require.NoError(t, err)

	req.Header.Set(wallet.TokenHeader, "uPldrToken")
	req.Header.Set("Content-Type", writer.FormDataContentType())

	publisher := &DummyPublisher{}
	handler := &Handler{UploadPath: os.TempDir()}

	provider := func(token, ip string) auth.Result {
		if token == "uPldrToken" {
			res := auth.NewResult(&models.User{ID: 20404}, nil)
			res.SDKAddress = "whatever"
			return res
		}
		return auth.NewResult(nil, errors.Base("error"))
	}

	rr := httptest.NewRecorder()
	auth.Middleware(provider)(http.HandlerFunc(handler.Handle)).ServeHTTP(rr, req)
	response := rr.Result()

	require.False(t, publisher.called)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	var rpcResponse jsonrpc.RPCResponse
	err = json.Unmarshal(rr.Body.Bytes(), &rpcResponse)
	require.NoError(t, err)
	assert.Equal(t, "unexpected EOF", rpcResponse.Error.Message)
	require.False(t, publisher.called)
}
