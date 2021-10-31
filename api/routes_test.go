package api

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/lbryio/lbrytv/app/publish"
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/apps/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoutesProxy(t *testing.T) {
	r := mux.NewRouter()
	rt := sdkrouter.New(config.GetLbrynetServers())

	req, err := http.NewRequest("POST", "/api/v1/proxy", bytes.NewBuffer([]byte(`{"method": "status"}`)))
	require.NoError(t, err)
	rr := httptest.NewRecorder()

	InstallRoutes(r, rt)
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"result":`)
}

func TestRoutesPublish(t *testing.T) {
	r := mux.NewRouter()
	rt := sdkrouter.New(config.GetLbrynetServers())

	req := publish.CreatePublishRequest(t, []byte("test file"))
	rr := httptest.NewRecorder()

	InstallRoutes(r, rt)
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	// Authentication Required error here is enough to see that the request
	// has been dispatched through the publish handler
	assert.Contains(t, rr.Body.String(), `"code": -32084`)
}

func TestCORS(t *testing.T) {
	r := mux.NewRouter()
	rt := sdkrouter.New(config.GetLbrynetServers())

	allowedDomains := []string{
		"https://odysee.com",
		"https://somedomain.com",
	}
	config.Override("CORSDomains", allowedDomains)

	InstallRoutes(r, rt)

	cases := map[string]string{
		"https://odysee.com":          "https://odysee.com",
		"https://somedomain.com":      "https://somedomain.com",
		"https://someotherdomain.com": "",
		"https://lbry.tv":             "",
	}

	defaultRequestHeaders := []string{
		"Content-Type",
		"X-Lbry-Auth-Token",
	}

	tusRequestHeaders := append(defaultRequestHeaders, publish.TusHeaders...)

	endpoints := []struct {
		method, path, headers string
	}{
		{http.MethodPost, "/api/v1/proxy", strings.Join(defaultRequestHeaders, ", ")},
		{http.MethodPost, "/api/v2/status", strings.Join(defaultRequestHeaders, ", ")},
		{http.MethodPost, "/api/v2/publish/", strings.Join(tusRequestHeaders, ", ")},
		{http.MethodHead, "/api/v2/publish/1", strings.Join(tusRequestHeaders, ", ")},
		{http.MethodPatch, "/api/v2/publish/1", strings.Join(tusRequestHeaders, ", ")},
		{http.MethodDelete, "/api/v2/publish/1", strings.Join(tusRequestHeaders, ", ")},
		{http.MethodPost, "/api/v2/publish/1/notify", strings.Join(tusRequestHeaders, ", ")},
	}

	for _, e := range endpoints {
		for orig, chost := range cases {
			t.Run(fmt.Sprintf("%s%s @ %s", e.method, e.path, orig), func(t *testing.T) {
				req, err := http.NewRequest(http.MethodOptions, e.path, nil)
				require.NoError(t, err)

				req.Header.Set("Origin", orig)
				req.Header.Set("Access-Control-Request-Headers", e.headers)
				req.Header.Set("Access-Control-Request-Method", e.method)

				rr := httptest.NewRecorder()

				r.ServeHTTP(rr, req)
				h := rr.Result().Header
				require.Equal(t, http.StatusOK, rr.Code)

				assert.Equal(t, chost, h.Get("Access-Control-Allow-Origin"))
				if chost != "" {
					assert.Equal(
						t,
						e.headers,
						h.Get("Access-Control-Allow-Headers"),
					)
				}
			})
		}
	}
}

func TestMiddlewareOrder(t *testing.T) {
	handler := func(i int) mux.MiddlewareFunc {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(fmt.Sprintf("%d", i)))
				next.ServeHTTP(w, r)
			})
		}
	}

	mw := middleware.Chain(
		handler(1),
		handler(2),
		handler(3),
		handler(4),
	)

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("5"))
	})

	r, err := http.NewRequest("GET", "/api/v1/proxy", nil)
	require.NoError(t, err)
	rr := httptest.NewRecorder()

	mw(finalHandler).ServeHTTP(rr, r)

	body, err := ioutil.ReadAll(rr.Result().Body)
	require.NoError(t, err)
	assert.Equal(t, "12345", string(body))
}
