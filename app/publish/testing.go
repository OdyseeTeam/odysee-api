package publish

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/lbryio/lbrytv/internal/lbrynet"

	"github.com/stretchr/testify/require"
)

// CreatePublishRequest creates and returns a HTTP request providing data for the publishing endpoint.
func CreatePublishRequest(t *testing.T, data []byte) *http.Request {
	readSeeker := bytes.NewReader(data)
	body := &bytes.Buffer{}

	writer := multipart.NewWriter(body)

	fileBody, err := writer.CreateFormFile(FileFieldName, "lbry_auto_test_file")
	require.NoError(t, err)
	_, err = io.Copy(fileBody, readSeeker)
	require.NoError(t, err)

	jsonPayload, err := writer.CreateFormField(JSONRPCFieldName)
	require.NoError(t, err)
	jsonPayload.Write([]byte(lbrynet.ExampleStreamCreateRequest))

	writer.Close()

	req, err := http.NewRequest("POST", "/api/v1/proxy", bytes.NewReader(body.Bytes()))
	require.NoError(t, err)

	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}
