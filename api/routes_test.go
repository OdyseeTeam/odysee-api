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
	"github.com/lbryio/lbrytv/config"
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

func TestRoutesOptions(t *testing.T) {
	r := mux.NewRouter()
	rt := sdkrouter.New(config.GetLbrynetServers())
	InstallRoutes(r, rt)

	for _, url := range []string{"/api/v1/proxy", "/api/v2/status"} {
		t.Run(url, func(t *testing.T) {
			req, err := http.NewRequest("OPTIONS", url, nil)
			require.NoError(t, err)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)
			h := rr.Result().Header
			assert.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, "7200", h.Get("Access-Control-Max-Age"))
			assert.Equal(t, "*", h.Get("Access-Control-Allow-Origin"))
			assert.Equal(
				t,
				"X-Lbry-Auth-Token, Origin, X-Requested-With, Content-Type, Accept",
				h.Get("Access-Control-Allow-Headers"),
			)
		})
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

	mw := middlewares(
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
