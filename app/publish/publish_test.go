package publish

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"

	"github.com/lbryio/lbrytv/app/users"
	"github.com/lbryio/lbrytv/internal/lbrynet"
	"github.com/ybbus/jsonrpc"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type DummyPublisher struct {
	called    bool
	filePath  string
	accountID string
	rawQuery  []byte
}

func (p *DummyPublisher) Publish(filePath, accountID string, rawQuery []byte) ([]byte, error) {
	p.called = true
	p.filePath = filePath
	p.accountID = accountID
	p.rawQuery = rawQuery
	return []byte(lbrynet.ExampleStreamCreateResponse), nil
}

func TestPublish(t *testing.T) {
	// hook := test.NewLocal(monitor.Logger)

	data := []byte("test file")
	readSeeker := bytes.NewReader(data)
	body := &bytes.Buffer{}

	writer := multipart.NewWriter(body)

	fileBody, err := writer.CreateFormFile(fileField, "lbry_auto_test_file")
	require.Nil(t, err)
	io.Copy(fileBody, readSeeker)

	jsonPayload, err := writer.CreateFormField(jsonrpcPayloadField)
	require.Nil(t, err)
	jsonPayload.Write([]byte(lbrynet.ExampleStreamCreateRequest))

	writer.Close()

	req, err := http.NewRequest("POST", "/", bytes.NewReader(body.Bytes()))
	require.Nil(t, err)

	req.Header.Set(users.TokenHeader, "uPldrToken")
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	authenticator := users.NewAuthenticator(&users.TestUserRetriever{AccountID: "UPldrAcc", Token: "uPldrToken"})
	publisher := &DummyPublisher{}
	pubHandler := NewUploadHandler(os.TempDir(), publisher)

	http.HandlerFunc(authenticator.Wrap(pubHandler.Handle)).ServeHTTP(rr, req)
	response := rr.Result()
	respBody, _ := ioutil.ReadAll(response.Body)

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, lbrynet.ExampleStreamCreateResponse, string(respBody))

	require.True(t, publisher.called)
	expectedPath := path.Join(os.TempDir(), "UPldrAcc", ".*_lbry_auto_test_file")
	assert.Regexp(t, expectedPath, publisher.filePath)
	assert.Equal(t, "UPldrAcc", publisher.accountID)
	assert.Equal(t, lbrynet.ExampleStreamCreateRequest, string(publisher.rawQuery))
	// logEntry := hook.LastEntry()
	// assert.Equal(t, "lbry_auth_test_file", logEntry.Data["file_name"])
}

func TestPublishAuthRequired(t *testing.T) {
	// hook := test.NewLocal(monitor.Logger)
	var rpcResponse jsonrpc.RPCResponse

	data := []byte("test file")
	readSeeker := bytes.NewReader(data)
	body := &bytes.Buffer{}

	writer := multipart.NewWriter(body)

	fileBody, err := writer.CreateFormFile(fileField, "lbry_auto_test_file")
	require.Nil(t, err)
	io.Copy(fileBody, readSeeker)

	jsonPayload, err := writer.CreateFormField(jsonrpcPayloadField)
	require.Nil(t, err)
	jsonPayload.Write([]byte(lbrynet.ExampleStreamCreateRequest))

	writer.Close()

	req, err := http.NewRequest("POST", "/", bytes.NewReader(body.Bytes()))
	require.Nil(t, err)

	// req.Header.Set(users.TokenHeader, "uPldrToken")
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	authenticator := users.NewAuthenticator(&users.TestUserRetriever{})
	publisher := &DummyPublisher{}
	pubHandler := NewUploadHandler(os.TempDir(), publisher)

	http.HandlerFunc(authenticator.Wrap(pubHandler.Handle)).ServeHTTP(rr, req)
	response := rr.Result()

	assert.Equal(t, http.StatusOK, response.StatusCode)
	err = json.Unmarshal(rr.Body.Bytes(), &rpcResponse)
	require.Nil(t, err)
	assert.Equal(t, "authentication required", rpcResponse.Error.Message)
	require.False(t, publisher.called)
	// logEntry := hook.LastEntry()
	// assert.Equal(t, "lbry_auth_test_file", logEntry.Data["file_name"])
}
