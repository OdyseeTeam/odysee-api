package publish

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"

	"github.com/lbryio/lbrytv/app/auth"
	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

type DummyPublisher struct {
	called   bool
	filePath string
	userID   int
	rawQuery []byte
}

func (p *DummyPublisher) Publish(filePath string, userID int, rawQuery []byte) []byte {
	p.called = true
	p.filePath = filePath
	p.userID = userID
	p.rawQuery = rawQuery
	return []byte(expectedStreamCreateResponse)
}

func TestUploadHandler(t *testing.T) {
	r := CreatePublishRequest(t, []byte("test file"))
	r.Header.Set(wallet.TokenHeader, "uPldrToken")

	publisher := &DummyPublisher{}
	handler := &Handler{
		Publisher:  publisher,
		UploadPath: os.TempDir(),
	}

	retriever := func(token, ip string) (*models.User, error) {
		if token == "uPldrToken" {
			return &models.User{ID: 20404}, nil
		}
		return nil, errors.New("error")
	}

	rr := httptest.NewRecorder()
	auth.Middleware(retriever)(http.HandlerFunc(handler.Handle)).ServeHTTP(rr, r)
	response := rr.Result()
	respBody, _ := ioutil.ReadAll(response.Body)

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, expectedStreamCreateResponse, string(respBody))

	require.True(t, publisher.called)
	expectedPath := path.Join(os.TempDir(), "20404", ".*_lbry_auto_test_file")
	assert.Regexp(t, expectedPath, publisher.filePath)
	assert.Equal(t, 20404, publisher.userID)
	assert.Equal(t, expectedStreamCreateRequest, string(publisher.rawQuery))

	_, err := os.Stat(publisher.filePath)
	assert.True(t, os.IsNotExist(err))
}

func TestUploadHandlerNoAuthMiddleware(t *testing.T) {
	r := CreatePublishRequest(t, []byte("test file"))
	r.Header.Set(wallet.TokenHeader, "uPldrToken")

	publisher := &DummyPublisher{}
	handler := &Handler{
		Publisher:  publisher,
		UploadPath: os.TempDir(),
	}

	rr := httptest.NewRecorder()
	assert.Panics(t, func() {
		handler.Handle(rr, r)
	})
}

func TestUploadHandlerAuthRequired(t *testing.T) {
	r := CreatePublishRequest(t, []byte("test file"))

	publisher := &DummyPublisher{}
	handler := &Handler{
		Publisher:  publisher,
		UploadPath: os.TempDir(),
	}

	retriever := func(token, ip string) (*models.User, error) {
		if token == "uPldrToken" {
			return &models.User{ID: 20404}, nil
		}
		return nil, errors.New("error")
	}

	rr := httptest.NewRecorder()
	auth.Middleware(retriever)(http.HandlerFunc(handler.Handle)).ServeHTTP(rr, r)
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
	jsonPayload.Write([]byte(expectedStreamCreateRequest))

	// <--- Not calling writer.Close() here to create an unexpected EOF

	req, err := http.NewRequest("POST", "/", bytes.NewReader(body.Bytes()))
	require.NoError(t, err)

	req.Header.Set(wallet.TokenHeader, "uPldrToken")
	req.Header.Set("Content-Type", writer.FormDataContentType())

	publisher := &DummyPublisher{}
	handler := &Handler{
		Publisher:  publisher,
		UploadPath: os.TempDir(),
	}

	retriever := func(token, ip string) (*models.User, error) {
		if token == "uPldrToken" {
			return &models.User{ID: 20404}, nil
		}
		return nil, errors.New("error")
	}

	rr := httptest.NewRecorder()
	auth.Middleware(retriever)(http.HandlerFunc(handler.Handle)).ServeHTTP(rr, req)
	response := rr.Result()

	require.False(t, publisher.called)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	var rpcResponse jsonrpc.RPCResponse
	err = json.Unmarshal(rr.Body.Bytes(), &rpcResponse)
	require.NoError(t, err)
	assert.Equal(t, "unexpected EOF", rpcResponse.Error.Message)
	require.False(t, publisher.called)
}
