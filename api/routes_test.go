package api

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
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

	for _, url := range []string{"/api/v1/proxy", "/api/v2/status"} {
		for orig, chost := range cases {
			t.Run(fmt.Sprintf("%v @ %v", url, orig), func(t *testing.T) {
				req, err := http.NewRequest(http.MethodOptions, url, nil)
				require.NoError(t, err)

				req.Header.Set("origin", orig)
				req.Header.Set("Access-Control-Request-Headers", "content-type,x-lbry-auth-token")
				req.Header.Set("Access-Control-Request-Method", http.MethodPost)

				rr := httptest.NewRecorder()

				r.ServeHTTP(rr, req)
				h := rr.Result().Header
				require.Equal(t, http.StatusOK, rr.Code)

				assert.Equal(t, chost, h.Get("Access-Control-Allow-Origin"))
				if chost != "" {
					assert.Equal(
						t,
						"Content-Type, X-Lbry-Auth-Token",
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
