package publish

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/OdyseeTeam/odysee-api/app/auth"
	"github.com/OdyseeTeam/odysee-api/app/query"
	"github.com/OdyseeTeam/odysee-api/app/sdkrouter"
	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/errors"
	"github.com/OdyseeTeam/odysee-api/internal/storage"
	"github.com/OdyseeTeam/odysee-api/internal/test"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/migrator"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tus/tusd/pkg/filelocker"
	"github.com/tus/tusd/pkg/filestore"
	tusd "github.com/tus/tusd/pkg/handler"
)

const tusVersion = "1.0.0"

func newTusTestCfg(uploadPath string) tusd.Config {
	composer := tusd.NewStoreComposer()
	store := filestore.New(uploadPath)
	store.UseIn(composer)
	locker := filelocker.New(uploadPath)
	locker.UseIn(composer)
	return tusd.Config{
		BasePath:      "/api/v2/publish/",
		StoreComposer: composer,
	}
}

func newTestTusHandler(t *testing.T) *TusHandler {
	t.Helper()

	uploadPath := t.TempDir()

	h, err := NewTusHandler(
		&wallet.TestAnyAuthenticator{},
		mockAuthProvider,
		newTusTestCfg(uploadPath),
		uploadPath,
	)
	assert.Nil(t, err)
	return h
}

func newTestTusHandlerWithOauth(t *testing.T, reqChan chan *test.Request) *TusHandler {
	if reqChan == nil {
		reqChan = test.ReqChan()
	}
	ts := test.MockHTTPServer(reqChan)

	uploadPath := t.TempDir()
	rt := sdkrouter.New(config.GetLbrynetServers())
	oAuther, err := wallet.NewOauthAuthenticator(config.GetOauthProviderURL(), config.GetOauthClientID(), config.GetInternalAPIHost(), rt)
	auther := wallet.NewPostProcessAuthenticator(oAuther, func(u *models.User) (*models.User, error) {
		u.R = u.R.NewStruct()
		u.R.LbrynetServer = &models.LbrynetServer{Address: ts.URL}
		return u, nil
	})
	require.NoError(t, err)

	go func() {
		ts.NextResponse <- expectedPublishResponse
	}()

	h, err := NewTusHandler(
		auther,
		mockAuthProvider,
		newTusTestCfg(uploadPath),
		uploadPath,
	)
	assert.Nil(t, err)
	return h
}

// tr is a helper to construct publish route path
var tr testRoute = "/api/v2/publish"

type testRoute string

func (tr testRoute) withID(id int) string {
	return fmt.Sprintf("%s/%d", tr, id)
}

func (tr testRoute) notify(id int) string {
	return fmt.Sprintf("%s/%d/notify", tr, id)
}

func (tr testRoute) root() string {
	return fmt.Sprintf("%s/", tr)
}

func mockAuthProvider(token, ip string) (*models.User, error) {
	reqChan := test.ReqChan()
	ts := test.MockHTTPServer(reqChan)
	var u *models.User
	if token == "legacyAuthToken123" {
		u = &models.User{ID: 20404}
		u.R = u.R.NewStruct()
		u.R.LbrynetServer = &models.LbrynetServer{Address: ts.URL}
	}
	go func() {
		ts.NextResponse <- expectedPublishResponse
	}()
	return u, nil
}

func newTestMux(h *TusHandler, mwf ...mux.MiddlewareFunc) *mux.Router {
	router := mux.NewRouter()

	testRouter := router.PathPrefix("/api/v2/publish").Subrouter()
	for _, fn := range mwf {
		testRouter.Use(fn)
	}
	testRouter.Use(h.Middleware)

	testRouter.HandleFunc("/", h.PostFile).Methods(http.MethodPost)
	testRouter.HandleFunc("/{id}", h.HeadFile).Methods(http.MethodHead)
	testRouter.HandleFunc("/{id}", h.PatchFile).Methods(http.MethodPatch)
	testRouter.HandleFunc("/{id}", h.DelFile).Methods(http.MethodDelete)
	testRouter.HandleFunc("/{id}/notify", h.Notify).Methods(http.MethodPost)

	return testRouter
}

type header struct {
	key, value string
}

func newPartialUpload(t *testing.T, h *TusHandler, headers ...header) string {
	t.Helper()

	testData := []byte("test file")
	r, err := http.NewRequest(http.MethodPost, tr.root(), bytes.NewReader(testData))
	assert.Nil(t, err)

	r.Header.Set("Upload-Length", fmt.Sprintf("%d", len(testData)))
	r.Header.Set("Tus-Resumable", tusVersion)

	for _, h := range headers {
		r.Header.Set(h.key, h.value)
	}

	w := httptest.NewRecorder()
	newTestMux(h).ServeHTTP(w, r)

	resp := w.Result()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	return resp.Header.Get("location")
}

func newFinalUpload(t *testing.T, h *TusHandler, headers ...header) string {
	loc := newPartialUpload(t, h, headers...)

	testData := []byte("test file")
	r, err := http.NewRequest(http.MethodPatch, loc, bytes.NewReader(testData))
	assert.Nil(t, err)

	r.Header.Set("Content-Type", "application/offset+octet-stream")
	r.Header.Set("Upload-Offset", "0")
	r.Header.Set("Tus-Resumable", tusVersion)

	w := httptest.NewRecorder()
	newTestMux(h).ServeHTTP(w, r)

	resp := w.Result()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	return loc
}

func TestNewTusHandler(t *testing.T) {
	auther, err := wallet.NewOauthAuthenticator(config.GetOauthProviderURL(), config.GetOauthClientID(), config.GetInternalAPIHost(), nil)
	require.NoError(t, err)

	successTestCases := []struct {
		name string
		fn   func() (auth.Authenticator, auth.Provider, tusd.Config, string)
	}{
		{
			name: "WithExistingDirectory",
			fn: func() (auth.Authenticator, auth.Provider, tusd.Config, string) {
				uploadPath := t.TempDir()
				return auther, mockAuthProvider, newTusTestCfg(uploadPath), uploadPath
			},
		},
		{
			name: "WithNewDirectory",
			fn: func() (auth.Authenticator, auth.Provider, tusd.Config, string) {
				uploadPath := filepath.Join(t.TempDir(), "new_dir")
				return auther, mockAuthProvider, newTusTestCfg(uploadPath), uploadPath
			},
		},
	}
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		for _, test := range successTestCases {
			test := test
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				h, err := NewTusHandler(test.fn())
				assert.Nil(t, err)
				assert.NotNil(t, h)
			})
		}
	})

	errorTestCases := []struct {
		name    string
		fn      func() (auth.Authenticator, auth.Provider, tusd.Config, string)
		wantErr error
	}{
		{
			name: "WithNilAuthProvider",
			fn: func() (auth.Authenticator, auth.Provider, tusd.Config, string) {
				uploadPath := t.TempDir()
				return nil, nil, newTusTestCfg(uploadPath), uploadPath
			},
		},
		{
			name: "WithRestrictedDirectoryAccess",
			fn: func() (auth.Authenticator, auth.Provider, tusd.Config, string) {
				if err := os.Mkdir("test_dir", 0444); err != nil { // read only
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Remove("test_dir"); err != nil {
						t.Fatal(err)
					}
				})
				uploadPath := filepath.Join("test_dir", "new_dir")
				return auther, mockAuthProvider, newTusTestCfg(uploadPath), uploadPath
			},
			wantErr: os.ErrPermission,
		},
	}
	t.Run("Error", func(t *testing.T) {
		t.Parallel()
		for _, test := range errorTestCases {
			test := test
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				_, err := NewTusHandler(test.fn())
				assert.NotNil(t, err)

				if test.wantErr != nil {
					if err = errors.Unwrap(err); !errors.Is(err, test.wantErr) {
						t.Fatalf("expecting error: %+v, got: %+v", test.wantErr, err)
					}
				}
			})
		}
	})
}

func TestNotify(t *testing.T) {
	db, dbCleanup, err := migrator.CreateTestDB(migrator.DBConfigFromApp(config.GetDatabase()), storage.MigrationsFS)
	if err != nil {
		panic(err)
	}
	storage.SetDB(db)
	config.Override("LbrynetServers", "")
	defer dbCleanup()

	token, err := wallet.GetTestTokenHeader()
	require.NoError(t, err)

	t.Run("FileNotExist", func(t *testing.T) {
		h := newTestTusHandlerWithOauth(t, nil)
		w := httptest.NewRecorder()

		r, err := http.NewRequest(http.MethodPost, tr.notify(404), nil)
		assert.Nil(t, err)

		r.Header.Set(wallet.AuthorizationHeader, token)
		r.Header.Set("Tus-Resumable", tusVersion)

		newTestMux(h).ServeHTTP(w, r)

		respBody, err := ioutil.ReadAll(w.Result().Body)
		assert.Nil(t, err)

		wantErrMsg := "no such file or directory"
		gotErrMsg := test.StrToRes(t, string(respBody)).Error.Message
		assert.Contains(t, gotErrMsg, wantErrMsg)
	})

	t.Run("UploadInProgress", func(t *testing.T) {
		h := newTestTusHandlerWithOauth(t, nil)
		w := httptest.NewRecorder()
		loc := newPartialUpload(t, h, header{wallet.AuthorizationHeader, token})

		r, err := http.NewRequest(http.MethodPost, loc+"/notify", http.NoBody)
		assert.Nil(t, err)

		r.Header.Set(wallet.AuthorizationHeader, token)
		r.Header.Set("Tus-Resumable", tusVersion)

		newTestMux(h).ServeHTTP(w, r)

		respBody, err := ioutil.ReadAll(w.Result().Body)
		assert.Nil(t, err)

		wantErrMsg := "upload is still in process"
		gotErrMsg := test.StrToRes(t, string(respBody)).Error.Message
		assert.Equal(t, wantErrMsg, gotErrMsg)

		f, err := os.Stat(filepath.Join(h.uploadPath, path.Base(loc)))
		assert.Nil(t, err)
		assert.NotNil(t, f)
	})

	t.Run("UploadCompleted", func(t *testing.T) {
		h := newTestTusHandlerWithOauth(t, nil)
		w := httptest.NewRecorder()

		loc := newFinalUpload(t, h,
			header{"Upload-Metadata", "filename ZHVtbXkubWQ="},
			header{wallet.AuthorizationHeader, token},
		)

		r, err := http.NewRequest(
			http.MethodPost, loc+"/notify",
			strings.NewReader(testPublishUpdateRequest),
		)
		assert.Nil(t, err)

		r.Header.Set(wallet.AuthorizationHeader, token)
		r.Header.Set("Tus-Resumable", tusVersion)

		newTestMux(h).ServeHTTP(w, r)

		response := w.Result()
		respBody, err := ioutil.ReadAll(response.Body)
		assert.Nil(t, err)

		test.AssertEqualJSON(t, expectedPublishResponse, respBody)

		f, err := os.Stat(filepath.Join(t.TempDir(), path.Base(loc)))
		assert.Nil(t, f)
		assert.NotNil(t, err)
	})

	t.Run("UpdateCompleted", func(t *testing.T) {
		reqChan := test.ReqChan()
		h := newTestTusHandlerWithOauth(t, reqChan)
		w := httptest.NewRecorder()

		loc := newFinalUpload(t, h,
			header{"Upload-Metadata", "filename ZHVtbXkubWQ="},
			header{wallet.AuthorizationHeader, token},
		)

		r, err := http.NewRequest(
			http.MethodPost, loc+"/notify",
			strings.NewReader(testPublishUpdateRequest),
		)
		assert.Nil(t, err)

		r.Header.Set(wallet.AuthorizationHeader, token)
		r.Header.Set("Tus-Resumable", tusVersion)

		newTestMux(h).ServeHTTP(w, r)

		response := w.Result()
		respBody, err := ioutil.ReadAll(response.Body)
		assert.Nil(t, err)

		test.AssertEqualJSON(t, expectedPublishResponse, respBody)

		f, err := os.Stat(filepath.Join(t.TempDir(), path.Base(loc)))
		assert.Nil(t, f)
		assert.NotNil(t, err)

		req := <-reqChan
		rpcReq := test.StrToReq(t, req.Body)
		params, ok := rpcReq.Params.(map[string]interface{})
		require.True(t, ok)
		require.Equal(t, query.MethodStreamUpdate, rpcReq.Method)
		assert.NotContains(t, "name", params)
		assert.True(t, params["replace"].(bool))
		assert.Equal(t, "f6d2070225511eeb8a1c33f1d4bdb76e22716547", params["claim_id"].(string))
	})

	t.Run("WithoutUploadMetadata", func(t *testing.T) {
		h := newTestTusHandlerWithOauth(t, nil)
		w := httptest.NewRecorder()

		loc := newFinalUpload(t, h, header{wallet.AuthorizationHeader, token})

		r, err := http.NewRequest(
			http.MethodPost, loc+"/notify",
			strings.NewReader(testPublishRequest),
		)
		assert.Nil(t, err)

		r.Header.Set(wallet.AuthorizationHeader, token)
		r.Header.Set("Tus-Resumable", tusVersion)

		newTestMux(h).ServeHTTP(w, r)

		respBody, err := ioutil.ReadAll(w.Result().Body)
		assert.Nil(t, err)

		wantErrMsg := "file metadata is required"
		gotErrMsg := test.StrToRes(t, string(respBody)).Error.Message
		assert.Equal(t, wantErrMsg, gotErrMsg)
	})
}

func TestTus(t *testing.T) {
	db, dbCleanup, err := migrator.CreateTestDB(migrator.DBConfigFromApp(config.GetDatabase()), storage.MigrationsFS)
	if err != nil {
		panic(err)
	}
	storage.SetDB(db)
	config.Override("LbrynetServers", "")
	defer dbCleanup()

	token, err := wallet.GetTestTokenHeader()
	require.NoError(t, err)

	t.Run("FailedToAuthorize", func(t *testing.T) {
		errAuthFn := func(token, ip string) (*models.User, error) {
			return nil, fmt.Errorf("failed to authorize")
		}

		uploadPath := t.TempDir()
		h, err := NewTusHandler(
			&wallet.TestMissingTokenAuthenticator{},
			errAuthFn,
			newTusTestCfg(uploadPath),
			uploadPath,
		)
		assert.Nil(t, err)

		w := httptest.NewRecorder()

		r, err := http.NewRequest(http.MethodPost, tr.root(), http.NoBody)
		assert.Nil(t, err)

		r.Header.Set(wallet.AuthorizationHeader, token)
		r.Header.Set("Tus-Resumable", tusVersion)

		newTestMux(h).ServeHTTP(w, r)

		resp := w.Result()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("WithoutTusVersionHeader", func(t *testing.T) {
		h := newTestTusHandlerWithOauth(t, nil)
		w := httptest.NewRecorder()

		testData := []byte("test file")
		r, err := http.NewRequest(http.MethodPost, tr.root(), bytes.NewReader(testData))
		assert.Nil(t, err)

		r.Header.Set(wallet.AuthorizationHeader, token)
		r.Header.Set("Upload-Length", fmt.Sprintf("%d", len(testData)))

		newTestMux(h).ServeHTTP(w, r)

		resp := w.Result()
		assert.Equal(t, http.StatusPreconditionFailed, resp.StatusCode)

		wantBody := "unsupported version\n"
		gotBody, err := ioutil.ReadAll(resp.Body)
		assert.Nil(t, err)
		assert.Equal(t, wantBody, string(gotBody))
	})

	t.Run("WithForwardProto", func(t *testing.T) {
		wantProto := "https"

		h := newTestTusHandler(t)
		w := httptest.NewRecorder()

		testData := []byte("test file")
		r, err := http.NewRequest(http.MethodPost, tr.root(), bytes.NewReader(testData))
		assert.Nil(t, err)

		r.Header.Set(wallet.AuthorizationHeader, token)
		r.Header.Set("Upload-Length", fmt.Sprintf("%d", len(testData)))
		r.Header.Set("Tus-Resumable", tusVersion)
		r.Header.Set("X-Forwarded-Proto", wantProto)

		newTestMux(h).ServeHTTP(w, r)

		resp := w.Result()
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		location := resp.Header.Get("location")
		assert.NotEmpty(t, location)

		u, err := url.Parse(location)
		assert.Nil(t, err)
		assert.Equal(t, wantProto, u.Scheme)
	})

	t.Run("CreateUpload", func(t *testing.T) {
		h := newTestTusHandlerWithOauth(t, nil)
		w := httptest.NewRecorder()

		testData := []byte("test file")
		r, err := http.NewRequest(http.MethodPost, tr.root(), bytes.NewReader(testData))
		assert.Nil(t, err)

		r.Header.Set(wallet.AuthorizationHeader, token)
		r.Header.Set("Upload-Length", fmt.Sprintf("%d", len(testData)))
		r.Header.Set("Tus-Resumable", tusVersion)

		newTestMux(h).ServeHTTP(w, r)

		resp := w.Result()
		assert.Equal(t, http.StatusCreated, resp.StatusCode, w.Body.String())
		assert.NotEmpty(t, resp.Header.Get("location"))
	})

	t.Run("ResumeUpload", func(t *testing.T) {
		h := newTestTusHandlerWithOauth(t, nil)

		loc := newPartialUpload(t, h,
			header{"Upload-Length", "3"},
			header{wallet.AuthorizationHeader, token},
		)

		tests := []struct {
			name           string
			offset         int
			wantStatusCode int
		}{
			{"ValidOffset", 0, http.StatusNoContent},
			{"MissmatchOffset", 4, http.StatusConflict},
		}

		testData := []byte("foo")
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				w := httptest.NewRecorder()
				r, err := http.NewRequest(http.MethodPatch, loc, bytes.NewReader(testData))
				assert.Nil(t, err)

				r.Header.Set(wallet.AuthorizationHeader, token)
				r.Header.Set("Content-Type", "application/offset+octet-stream")
				r.Header.Set("Upload-Offset", strconv.Itoa(test.offset))
				r.Header.Set("Tus-Resumable", tusVersion)

				newTestMux(h).ServeHTTP(w, r)

				resp := w.Result()
				assert.Equal(t, test.wantStatusCode, resp.StatusCode)
			})
		}
	})

	t.Run("ResumeWithChunks", func(t *testing.T) {
		h := newTestTusHandlerWithOauth(t, nil)

		loc := newPartialUpload(t, h,
			header{"Upload-Length", "6"},
			header{wallet.AuthorizationHeader, token},
		)

		testData := []byte("foobar")
		b := bytes.NewReader(testData)

		const chunkSize = 2
		for i := 0; i < b.Len(); i += chunkSize {
			t.Run(fmt.Sprintf("PatchOffset-%d", i), func(t *testing.T) {
				buf := make([]byte, chunkSize)
				if _, err := b.ReadAt(buf, int64(i)); err != nil {
					t.Fatal(err)
				}

				w := httptest.NewRecorder()
				r, err := http.NewRequest(http.MethodPatch, loc, bytes.NewReader(buf))
				assert.Nil(t, err)

				r.Header.Set(wallet.AuthorizationHeader, token)
				r.Header.Set("Content-Type", "application/offset+octet-stream")
				r.Header.Set("Upload-Offset", strconv.Itoa(i))
				r.Header.Set("Tus-Resumable", tusVersion)

				newTestMux(h).ServeHTTP(w, r)

				resp := w.Result()
				assert.Equal(t, http.StatusNoContent, resp.StatusCode)
			})
		}
	})

	t.Run("DeleteUpload", func(t *testing.T) {
		h := newTestTusHandlerWithOauth(t, nil)
		w := httptest.NewRecorder()

		loc := newPartialUpload(t, h, header{wallet.AuthorizationHeader, token})

		r, err := http.NewRequest(http.MethodDelete, loc, http.NoBody)
		assert.Nil(t, err)

		r.Header.Set(wallet.AuthorizationHeader, token)
		r.Header.Set("Tus-Resumable", tusVersion)

		newTestMux(h).ServeHTTP(w, r)

		resp := w.Result()
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)

		f, err := os.Stat(filepath.Join(os.TempDir(), path.Base(loc)))
		assert.Nil(t, f)
		assert.NotNil(t, err)
	})

	t.Run("QueryFile", func(t *testing.T) {
		h := newTestTusHandlerWithOauth(t, nil)
		w := httptest.NewRecorder()

		loc := newPartialUpload(t, h, header{wallet.AuthorizationHeader, token})

		r, err := http.NewRequest(http.MethodHead, loc, http.NoBody)
		assert.Nil(t, err)

		r.Header.Set(wallet.AuthorizationHeader, token)
		r.Header.Set("Tus-Resumable", tusVersion)

		newTestMux(h).ServeHTTP(w, r)

		resp := w.Result()
		data, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode, string(data))
	})

	t.Run("QueryNonExistentFile", func(t *testing.T) {
		h := newTestTusHandlerWithOauth(t, nil)
		w := httptest.NewRecorder()
		_ = newPartialUpload(t, h, header{wallet.AuthorizationHeader, token})

		r, err := http.NewRequest(http.MethodHead, tr.withID(404), http.NoBody)
		assert.Nil(t, err)

		r.Header.Set(wallet.AuthorizationHeader, token)
		r.Header.Set("Tus-Resumable", tusVersion)

		newTestMux(h).ServeHTTP(w, r)

		resp := w.Result()

		data, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, string(data))
	})
}

// TODO: Remove this after legacy tokens go away.

func TestNotifyLegacy(t *testing.T) {
	db, dbCleanup, err := migrator.CreateTestDB(migrator.DBConfigFromApp(config.GetDatabase()), storage.MigrationsFS)
	if err != nil {
		panic(err)
	}
	storage.SetDB(db)
	config.Override("LbrynetServers", "")
	defer dbCleanup()

	auther, err := wallet.NewOauthAuthenticator(config.GetOauthProviderURL(), config.GetOauthClientID(), config.GetInternalAPIHost(), nil)
	require.NoError(t, err)

	t.Run("FileNotExist", func(t *testing.T) {
		h := newTestTusHandlerWithOauth(t, nil)
		w := httptest.NewRecorder()

		r, err := http.NewRequest(http.MethodPost, tr.notify(404), nil)
		assert.Nil(t, err)

		r.Header.Set(wallet.LegacyTokenHeader, "legacyAuthToken123")
		r.Header.Set("Tus-Resumable", tusVersion)

		newTestMux(h).ServeHTTP(w, r)

		respBody, err := ioutil.ReadAll(w.Result().Body)
		assert.Nil(t, err)

		wantErrMsg := "no such file or directory"
		gotErrMsg := test.StrToRes(t, string(respBody)).Error.Message
		assert.Contains(t, gotErrMsg, wantErrMsg)
	})

	t.Run("UploadInProgress", func(t *testing.T) {
		uploadPath := t.TempDir()

		h, err := NewTusHandler(
			auther,
			mockAuthProvider,
			newTusTestCfg(uploadPath),
			uploadPath,
		)
		assert.Nil(t, err)

		w := httptest.NewRecorder()
		loc := newPartialUpload(t, h, header{wallet.LegacyTokenHeader, "legacyAuthToken123"})

		r, err := http.NewRequest(http.MethodPost, loc+"/notify", http.NoBody)
		assert.Nil(t, err)

		r.Header.Set(wallet.LegacyTokenHeader, "legacyAuthToken123")
		r.Header.Set("Tus-Resumable", tusVersion)

		newTestMux(h, auth.LegacyMiddleware(mockAuthProvider)).ServeHTTP(w, r)

		respBody, err := ioutil.ReadAll(w.Result().Body)
		assert.Nil(t, err)

		wantErrMsg := "upload is still in process"
		gotErrMsg := test.StrToRes(t, string(respBody)).Error.Message
		assert.Equal(t, wantErrMsg, gotErrMsg)

		f, err := os.Stat(filepath.Join(uploadPath, path.Base(loc)))
		assert.Nil(t, err)
		assert.NotNil(t, f)
	})

	t.Run("UploadCompleted", func(t *testing.T) {
		h := newTestTusHandlerWithOauth(t, nil)
		w := httptest.NewRecorder()

		loc := newFinalUpload(t, h,
			header{"Upload-Metadata", "filename ZHVtbXkubWQ="},
			header{wallet.LegacyTokenHeader, "legacyAuthToken123"},
		)

		r, err := http.NewRequest(
			http.MethodPost, loc+"/notify",
			strings.NewReader(testPublishRequest),
		)
		assert.Nil(t, err)

		r.Header.Set(wallet.LegacyTokenHeader, "legacyAuthToken123")
		r.Header.Set("Tus-Resumable", tusVersion)

		newTestMux(h, auth.LegacyMiddleware(mockAuthProvider)).ServeHTTP(w, r)

		response := w.Result()
		respBody, err := ioutil.ReadAll(response.Body)
		assert.Nil(t, err)

		test.AssertEqualJSON(t, expectedPublishResponse, respBody)

		f, err := os.Stat(filepath.Join(t.TempDir(), path.Base(loc)))
		assert.Nil(t, f)
		assert.NotNil(t, err)
	})

	t.Run("UpdateCompleted", func(t *testing.T) {
		h := newTestTusHandlerWithOauth(t, nil)
		w := httptest.NewRecorder()

		loc := newFinalUpload(t, h,
			header{"Upload-Metadata", "filename ZHVtbXkubWQ="},
			header{wallet.LegacyTokenHeader, "legacyAuthToken123"},
		)

		r, err := http.NewRequest(
			http.MethodPost, loc+"/notify",
			strings.NewReader(testPublishUpdateRequest),
		)
		assert.Nil(t, err)

		r.Header.Set(wallet.LegacyTokenHeader, "legacyAuthToken123")
		r.Header.Set("Tus-Resumable", tusVersion)

		newTestMux(h, auth.LegacyMiddleware(mockAuthProvider)).ServeHTTP(w, r)

		response := w.Result()
		respBody, err := ioutil.ReadAll(response.Body)
		assert.Nil(t, err)

		test.AssertEqualJSON(t, expectedPublishResponse, respBody)

		f, err := os.Stat(filepath.Join(t.TempDir(), path.Base(loc)))
		assert.Nil(t, f)
		assert.NotNil(t, err)
	})

	t.Run("WithoutUploadMetadata", func(t *testing.T) {
		h := newTestTusHandlerWithOauth(t, nil)
		w := httptest.NewRecorder()

		loc := newFinalUpload(t, h, header{wallet.LegacyTokenHeader, "legacyAuthToken123"})

		r, err := http.NewRequest(
			http.MethodPost, loc+"/notify",
			strings.NewReader(testPublishRequest),
		)
		assert.Nil(t, err)

		r.Header.Set(wallet.LegacyTokenHeader, "legacyAuthToken123")
		r.Header.Set("Tus-Resumable", tusVersion)

		newTestMux(h, auth.LegacyMiddleware(mockAuthProvider)).ServeHTTP(w, r)

		respBody, err := ioutil.ReadAll(w.Result().Body)
		assert.Nil(t, err)

		wantErrMsg := "file metadata is required"
		gotErrMsg := test.StrToRes(t, string(respBody)).Error.Message
		assert.Equal(t, wantErrMsg, gotErrMsg)
	})
}

// TODO: This is for testing legacy token authentication. Remove this after legacy tokens go away.
func TestTusLegacyToken(t *testing.T) {
	db, dbCleanup, err := migrator.CreateTestDB(migrator.DBConfigFromApp(config.GetDatabase()), storage.MigrationsFS)
	if err != nil {
		panic(err)
	}
	storage.SetDB(db)
	config.Override("LbrynetServers", "")
	defer dbCleanup()

	t.Run("FailedToAuthorize", func(t *testing.T) {
		errAuthFn := func(token, ip string) (*models.User, error) {
			return nil, fmt.Errorf("failed to authorize")
		}

		uploadPath := t.TempDir()
		h, err := NewTusHandler(
			&wallet.TestMissingTokenAuthenticator{},
			errAuthFn,
			newTusTestCfg(uploadPath),
			uploadPath,
		)
		assert.Nil(t, err)

		w := httptest.NewRecorder()

		r, err := http.NewRequest(http.MethodPost, tr.root(), http.NoBody)
		assert.Nil(t, err)

		r.Header.Set(wallet.LegacyTokenHeader, "legacyAuthToken123")
		r.Header.Set("Tus-Resumable", tusVersion)

		newTestMux(h).ServeHTTP(w, r)

		resp := w.Result()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("WithoutTusVersionHeader", func(t *testing.T) {
		h := newTestTusHandlerWithOauth(t, nil)
		w := httptest.NewRecorder()

		testData := []byte("test file")
		r, err := http.NewRequest(http.MethodPost, tr.root(), bytes.NewReader(testData))
		assert.Nil(t, err)

		r.Header.Set(wallet.LegacyTokenHeader, "legacyAuthToken123")
		r.Header.Set("Upload-Length", fmt.Sprintf("%d", len(testData)))

		newTestMux(h).ServeHTTP(w, r)

		resp := w.Result()
		assert.Equal(t, http.StatusPreconditionFailed, resp.StatusCode)

		wantBody := "unsupported version\n"
		gotBody, err := ioutil.ReadAll(resp.Body)
		assert.Nil(t, err)
		assert.Equal(t, wantBody, string(gotBody))
	})

	t.Run("WithForwardProto", func(t *testing.T) {
		wantProto := "https"

		h := newTestTusHandlerWithOauth(t, nil)
		w := httptest.NewRecorder()

		testData := []byte("test file")
		r, err := http.NewRequest(http.MethodPost, tr.root(), bytes.NewReader(testData))
		assert.Nil(t, err)

		r.Header.Set(wallet.LegacyTokenHeader, "legacyAuthToken123")
		r.Header.Set("Upload-Length", fmt.Sprintf("%d", len(testData)))
		r.Header.Set("Tus-Resumable", tusVersion)
		r.Header.Set("X-Forwarded-Proto", wantProto)

		newTestMux(h).ServeHTTP(w, r)

		resp := w.Result()
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		location := resp.Header.Get("location")
		assert.NotEmpty(t, location)

		u, err := url.Parse(location)
		assert.Nil(t, err)
		assert.Equal(t, wantProto, u.Scheme)
	})

	t.Run("CreateUpload", func(t *testing.T) {
		h := newTestTusHandlerWithOauth(t, nil)
		w := httptest.NewRecorder()

		testData := []byte("test file")
		r, err := http.NewRequest(http.MethodPost, tr.root(), bytes.NewReader(testData))
		assert.Nil(t, err)

		r.Header.Set(wallet.LegacyTokenHeader, "legacyAuthToken123")
		r.Header.Set("Upload-Length", fmt.Sprintf("%d", len(testData)))
		r.Header.Set("Tus-Resumable", tusVersion)

		newTestMux(h).ServeHTTP(w, r)

		resp := w.Result()
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.NotEmpty(t, resp.Header.Get("location"))
	})

	t.Run("ResumeUpload", func(t *testing.T) {
		h := newTestTusHandlerWithOauth(t, nil)

		loc := newPartialUpload(t, h,
			header{"Upload-Length", "3"},
			header{wallet.LegacyTokenHeader, "legacyAuthToken123"},
		)

		tests := []struct {
			name           string
			offset         int
			wantStatusCode int
		}{
			{"ValidOffset", 0, http.StatusNoContent},
			{"MissmatchOffset", 4, http.StatusConflict},
		}

		testData := []byte("foo")
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				w := httptest.NewRecorder()
				r, err := http.NewRequest(http.MethodPatch, loc, bytes.NewReader(testData))
				assert.Nil(t, err)

				r.Header.Set(wallet.LegacyTokenHeader, "legacyAuthToken123")
				r.Header.Set("Content-Type", "application/offset+octet-stream")
				r.Header.Set("Upload-Offset", strconv.Itoa(test.offset))
				r.Header.Set("Tus-Resumable", tusVersion)

				newTestMux(h).ServeHTTP(w, r)

				resp := w.Result()
				assert.Equal(t, test.wantStatusCode, resp.StatusCode)
			})
		}
	})

	t.Run("ResumeWithChunks", func(t *testing.T) {
		h := newTestTusHandlerWithOauth(t, nil)

		loc := newPartialUpload(t, h,
			header{"Upload-Length", "6"},
			header{wallet.LegacyTokenHeader, "legacyAuthToken123"},
		)

		testData := []byte("foobar")
		b := bytes.NewReader(testData)

		const chunkSize = 2
		for i := 0; i < b.Len(); i += chunkSize {
			t.Run(fmt.Sprintf("PatchOffset-%d", i), func(t *testing.T) {
				buf := make([]byte, chunkSize)
				if _, err := b.ReadAt(buf, int64(i)); err != nil {
					t.Fatal(err)
				}

				w := httptest.NewRecorder()
				r, err := http.NewRequest(http.MethodPatch, loc, bytes.NewReader(buf))
				assert.Nil(t, err)

				r.Header.Set(wallet.LegacyTokenHeader, "legacyAuthToken123")
				r.Header.Set("Content-Type", "application/offset+octet-stream")
				r.Header.Set("Upload-Offset", strconv.Itoa(i))
				r.Header.Set("Tus-Resumable", tusVersion)

				newTestMux(h).ServeHTTP(w, r)

				resp := w.Result()
				assert.Equal(t, http.StatusNoContent, resp.StatusCode)
			})
		}
	})

	t.Run("DeleteUpload", func(t *testing.T) {
		h := newTestTusHandlerWithOauth(t, nil)
		w := httptest.NewRecorder()

		loc := newPartialUpload(t, h, header{wallet.LegacyTokenHeader, "legacyAuthToken123"})

		r, err := http.NewRequest(http.MethodDelete, loc, http.NoBody)
		assert.Nil(t, err)

		r.Header.Set(wallet.LegacyTokenHeader, "legacyAuthToken123")
		r.Header.Set("Tus-Resumable", tusVersion)

		newTestMux(h).ServeHTTP(w, r)

		resp := w.Result()
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)

		f, err := os.Stat(filepath.Join(os.TempDir(), path.Base(loc)))
		assert.Nil(t, f)
		assert.NotNil(t, err)
	})

	t.Run("QueryFile", func(t *testing.T) {
		h := newTestTusHandlerWithOauth(t, nil)
		w := httptest.NewRecorder()

		loc := newPartialUpload(t, h, header{wallet.LegacyTokenHeader, "legacyAuthToken123"})

		r, err := http.NewRequest(http.MethodHead, loc, http.NoBody)
		assert.Nil(t, err)

		r.Header.Set(wallet.LegacyTokenHeader, "legacyAuthToken123")
		r.Header.Set("Tus-Resumable", tusVersion)

		newTestMux(h).ServeHTTP(w, r)

		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("QueryNonExistentFile", func(t *testing.T) {
		h := newTestTusHandlerWithOauth(t, nil)
		w := httptest.NewRecorder()
		_ = newPartialUpload(t, h, header{wallet.LegacyTokenHeader, "legacyAuthToken123"})

		r, err := http.NewRequest(http.MethodHead, tr.withID(404), http.NoBody)
		assert.Nil(t, err)

		r.Header.Set(wallet.LegacyTokenHeader, "legacyAuthToken123")
		r.Header.Set("Tus-Resumable", tusVersion)

		newTestMux(h).ServeHTTP(w, r)

		resp := w.Result()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
