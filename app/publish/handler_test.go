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
	"strings"
	"testing"

	"github.com/lbryio/lbrytv/app/auth"
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/app/wallet"
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

	provider := func(token, ip string) (*models.User, error) {
		var u *models.User
		if token == "uPldrToken" {
			u = &models.User{ID: 20404}
			u.R = u.R.NewStruct()
			u.R.LbrynetServer = &models.LbrynetServer{Address: ts.URL}
		}
		return u, nil
	}

	rr := httptest.NewRecorder()
	auth.Middleware(provider)(http.HandlerFunc(handler.Handle)).ServeHTTP(rr, r)
	response := rr.Result()
	respBody, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, response.StatusCode)
	test.AssertEqualJSON(t, expectedStreamCreateResponse, respBody)

	require.True(t, publisher.called)
	expectedPath := path.Join(os.TempDir(), "20404", ".+", "lbry_auto_test_file")
	assert.Regexp(t, expectedPath, publisher.filePath)
	assert.Equal(t, sdkrouter.WalletID(20404), publisher.walletID)
	expectedReq := fmt.Sprintf(expectedStreamCreateRequest, sdkrouter.WalletID(20404), publisher.filePath)
	test.AssertEqualJSON(t, expectedReq, publisher.rawQuery)

	_, err = os.Stat(publisher.filePath)
	assert.True(t, os.IsNotExist(err))
}

func TestHandler_NoAuthMiddleware(t *testing.T) {
	r, err := http.NewRequest("POST", "/api/v1/proxy", &bytes.Buffer{})
	require.NoError(t, err)
	r.Header.Set(wallet.TokenHeader, "uPldrToken")

	handler := &Handler{UploadPath: os.TempDir()}

	rr := httptest.NewRecorder()
	handler.Handle(rr, r)
	respBody, err := ioutil.ReadAll(rr.Result().Body)
	require.NoError(t, err)
	assert.Equal(t, "auth.Middleware is required", test.StrToRes(t, string(respBody)).Error.Message)
}

func TestHandler_NoSDKAddress(t *testing.T) {
	r := CreatePublishRequest(t, []byte("test file"))
	r.Header.Set(wallet.TokenHeader, "x")
	rr := httptest.NewRecorder()

	handler := &Handler{UploadPath: os.TempDir()}
	provider := func(token, ip string) (*models.User, error) {
		return &models.User{ID: 20404}, nil
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

	provider := func(token, ip string) (*models.User, error) {
		if token == "uPldrToken" {
			return &models.User{ID: 20404}, nil
		}
		return nil, nil
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

	provider := func(token, ip string) (*models.User, error) {
		var u *models.User
		if token == "uPldrToken" {
			u = &models.User{ID: 20404}
			u.R = u.R.NewStruct()
			u.R.LbrynetServer = &models.LbrynetServer{Address: "whatever"}
		}
		return u, nil
	}

	rr := httptest.NewRecorder()
	auth.Middleware(provider)(http.HandlerFunc(handler.Handle)).ServeHTTP(rr, req)
	response := rr.Result()

	require.False(t, publisher.called)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	var rpcResponse jsonrpc.RPCResponse
	err = json.Unmarshal(rr.Body.Bytes(), &rpcResponse)
	require.NoError(t, err)
	assert.Equal(t, "request error: unexpected EOF", rpcResponse.Error.Message)
	require.False(t, publisher.called)
}

func Test_fetchFileInvalidInput(t *testing.T) {
	h := &Handler{UploadPath: os.TempDir()}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "404") {
			w.WriteHeader(http.StatusNotFound)
		} else if strings.HasSuffix(r.URL.Path, "400") {
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()

	cases := []struct {
		url, errMsg string
	}{
		{"", ErrEmptyRemoteURL.Error()},
		{fmt.Sprintf("%v/files/404", ts.URL), "remote server returned non-OK status 404"},
		{fmt.Sprintf("%v/files/400", ts.URL), "remote server returned non-OK status 400"},
		{"/etc/passwd", `Get "/etc/passwd": unsupported protocol scheme ""`},
		{"http://nonexistenthost/some_file.mp4", `dial tcp: lookup nonexistenthost:`},
		{"http://nonexistenthost/", "couldn't determine remote file name"},
		{ts.URL, `couldn't determine remote file name`},
		{"/", "couldn't determine remote file name"},
	}

	for _, c := range cases {
		t.Run(c.url, func(t *testing.T) {
			r := CreatePublishRequest(t, nil, FormParam{remoteURLParam, c.url})

			f, err := h.fetchFile(r, 20404)
			assert.NotNil(t, err)
			assert.Nil(t, f)
			assert.Regexp(t, fmt.Sprintf(".*%v.*", c.errMsg), err.Error())
		})
	}
}

func Test_fetchFile(t *testing.T) {
	h := &Handler{UploadPath: os.TempDir()}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "1Mb.dat") {
			w.Header().Add("content-length", "125000")
			for range [125000]int{} {
				w.Write([]byte{0})
			}
		} else if strings.HasSuffix(r.URL.Path, "../../etc/passwd") {
			w.Write([]byte{0})
		}
	}))
	defer ts.Close()

	cases := []struct {
		url, nameRe string
		size        int64
	}{
		{fmt.Sprintf("%v/1Mb.dat", ts.URL), ".+/20404/.+", 125000},
		{fmt.Sprintf("%v/../../../../../etc/passwd", ts.URL), ".+/20404/.+/passwd$", 1},
	}

	for _, c := range cases {
		t.Run(c.url, func(t *testing.T) {
			r := CreatePublishRequest(t, nil, FormParam{remoteURLParam, c.url})

			f, err := h.fetchFile(r, 20404)
			require.NoError(t, err)
			assert.Regexp(t, c.nameRe, f.Name())
			s, err := os.Stat(f.Name())
			require.NoError(t, err)
			require.EqualValues(t, c.size, s.Size())
		})
	}
}

func Test_fetchFileWithRetries(t *testing.T) {
	t.Parallel()

	h := &Handler{UploadPath: os.TempDir()}

	t.Run("Success", func(t *testing.T) {
		t.Parallel()

		attempt := 1
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if attempt == 2 {
				w.WriteHeader(http.StatusOK)
				w.Header().Add("content-length", "125000")
				for range [125000]int{} {
					w.Write([]byte{0})
				}
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
			attempt++
		}))
		defer ts.Close()

		r := CreatePublishRequest(t, nil, FormParam{
			remoteURLParam,
			fmt.Sprintf("%v/with_retries/success", ts.URL),
		})

		_, err := h.fetchFile(r, 20404)
		require.NoError(t, err)
	})

	t.Run("FailedWithMaximumRetries", func(t *testing.T) {
		t.Parallel()

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(600)
		}))
		defer ts.Close()

		r := CreatePublishRequest(t, nil, FormParam{
			remoteURLParam,
			fmt.Sprintf("%v/bad_status_code", ts.URL),
		})
		f, err := h.fetchFile(r, 20404)

		assert.NotNil(t, err)
		assert.Nil(t, f)
		assert.Contains(t, err.Error(), "remote server returned non-OK status 600")
	})

	t.Run("FailedWithClosedConnection", func(t *testing.T) {
		t.Parallel()

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
		}))
		defer ts.Close()

		r := CreatePublishRequest(t, nil, FormParam{
			remoteURLParam,
			fmt.Sprintf("%v/closed_connection", ts.URL),
		})

		// close the listener to simulate server closing the client connection
		if err := ts.Listener.Close(); err != nil {
			t.Fatalf("failed to close client connection: %v", err)
		}

		f, err := h.fetchFile(r, 20404)

		assert.NotNil(t, err)
		assert.Nil(t, f)
		assert.Contains(t, err.Error(), "connect: connection refused")
	})
}
