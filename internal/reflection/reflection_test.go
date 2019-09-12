package reflection

// import (
// 	"bytes"
// 	"encoding/json"
// 	"flag"
// 	"io"
// 	"io/ioutil"
// 	"mime/multipart"
// 	"net/http"
// 	"net/http/httptest"
// 	"os"
// 	"path"
// 	"testing"

// 	"github.com/lbryio/lbrytv/config"
// 	"github.com/lbryio/lbrytv/app/publish"
// 	"github.com/lbryio/lbrytv/app/users"
// 	"github.com/lbryio/lbrytv/internal/lbrynet"

// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/require"
// 	"github.com/ybbus/jsonrpc"
// )

// var pubFile string
// var pubAccountID string

// func init() {
// 	flag.StringVar(&pubFile, "pubFile", "", "path to a file for publishing and reflection")
// 	flag.StringVar(&pubAccountID, "pubAccountID", "", "ID of publisher's account (has to have funds)")
// 	flag.Parse()
// }

// func TestUploadHandler(t *testing.T) {
// 	file, err := os.Open(pubFile)
// 	body := &bytes.Buffer{}

// 	writer := multipart.NewWriter(body)

// 	fileBody, err := writer.CreateFormFile(publish.FileFieldName, "lbry_auto_test_file")
// 	require.Nil(t, err)
// 	_, err = io.Copy(fileBody, file)
// 	require.Nil(t, err)

// 	jsonPayload, err := writer.CreateFormField(publish.JSONRPCFieldName)
// 	require.Nil(t, err)
// 	jsonPayload.Write([]byte(lbrynet.ExampleStreamCreateRequest))

// 	writer.Close()

// 	req, err := http.NewRequest("POST", "/", bytes.NewReader(body.Bytes()))
// 	require.Nil(t, err)

// 	req.Header.Set(users.TokenHeader, "any")
// 	req.Header.Set("Content-Type", writer.FormDataContentType())

// 	rr := httptest.NewRecorder()
// 	authenticator := users.NewAuthenticator(&users.TestUserRetriever{AccountID: pubAccountID})
// 	publisher := &publish.LbrynetPublisher{}
// 	pubHandler := publish.NewUploadHandler(os.TempDir(), publisher)

// 	http.HandlerFunc(authenticator.Wrap(pubHandler.Handle)).ServeHTTP(rr, req)
// 	response := rr.Result()
// 	respBody, _ := ioutil.ReadAll(response.Body)

// 	assert.Equal(t, http.StatusOK, response.StatusCode)

// 	require.True(t, publisher.called)
// 	expectedPath := path.Join(os.TempDir(), "UPldrAcc", ".*_lbry_auto_test_file")
// 	assert.Regexp(t, expectedPath, publisher.filePath)
// 	assert.Equal(t, "UPldrAcc", publisher.accountID)
// 	assert.Equal(t, lbrynet.ExampleStreamCreateRequest, string(publisher.rawQuery))
// }

// func TestUploadHandlerAuthRequired(t *testing.T) {
// 	var rpcResponse jsonrpc.RPCResponse

// 	data := []byte("test file")
// 	readSeeker := bytes.NewReader(data)
// 	body := &bytes.Buffer{}

// 	writer := multipart.NewWriter(body)

// 	fileBody, err := writer.CreateFormFile(FileFieldName, "lbry_auto_test_file")
// 	require.Nil(t, err)
// 	io.Copy(fileBody, readSeeker)

// 	jsonPayload, err := writer.CreateFormField(JSONRPCFieldName)
// 	require.Nil(t, err)
// 	jsonPayload.Write([]byte(lbrynet.ExampleStreamCreateRequest))

// 	writer.Close()

// 	req, err := http.NewRequest("POST", "/", bytes.NewReader(body.Bytes()))
// 	require.Nil(t, err)

// 	req.Header.Set("Content-Type", writer.FormDataContentType())

// 	rr := httptest.NewRecorder()
// 	authenticator := users.NewAuthenticator(&users.TestUserRetriever{})
// 	publisher := &DummyPublisher{}
// 	pubHandler := NewUploadHandler(os.TempDir(), publisher)

// 	http.HandlerFunc(authenticator.Wrap(pubHandler.Handle)).ServeHTTP(rr, req)
// 	response := rr.Result()

// 	assert.Equal(t, http.StatusOK, response.StatusCode)
// 	err = json.Unmarshal(rr.Body.Bytes(), &rpcResponse)
// 	require.Nil(t, err)
// 	assert.Equal(t, "authentication required", rpcResponse.Error.Message)
// 	require.False(t, publisher.called)
// }

// func TestUploadHandlerSystemError(t *testing.T) {
// 	var rpcResponse jsonrpc.RPCResponse

// 	data := []byte("test file")
// 	readSeeker := bytes.NewReader(data)
// 	body := &bytes.Buffer{}

// 	writer := multipart.NewWriter(body)

// 	fileBody, err := writer.CreateFormFile(FileFieldName, "lbry_auto_test_file")
// 	require.Nil(t, err)
// 	io.Copy(fileBody, readSeeker)

// 	jsonPayload, err := writer.CreateFormField(JSONRPCFieldName)
// 	require.Nil(t, err)
// 	jsonPayload.Write([]byte(lbrynet.ExampleStreamCreateRequest))

// 	// Not calling writer.Close() here to create unexpected EOF

// 	req, err := http.NewRequest("POST", "/", bytes.NewReader(body.Bytes()))
// 	require.Nil(t, err)

// 	req.Header.Set(users.TokenHeader, "uPldrToken")
// 	req.Header.Set("Content-Type", writer.FormDataContentType())

// 	rr := httptest.NewRecorder()
// 	authenticator := users.NewAuthenticator(&users.TestUserRetriever{AccountID: "UPldrAcc", Token: "uPldrToken"})
// 	publisher := &DummyPublisher{}
// 	pubHandler := NewUploadHandler(os.TempDir(), publisher)

// 	http.HandlerFunc(authenticator.Wrap(pubHandler.Handle)).ServeHTTP(rr, req)
// 	response := rr.Result()

// 	require.False(t, publisher.called)
// 	assert.Equal(t, http.StatusOK, response.StatusCode)
// 	err = json.Unmarshal(rr.Body.Bytes(), &rpcResponse)
// 	require.Nil(t, err)
// 	assert.Equal(t, "unexpected EOF", rpcResponse.Error.Message)
// 	require.False(t, publisher.called)
// }
