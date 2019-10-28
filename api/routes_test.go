package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lbryio/lbrytv/app/proxy"
	"github.com/lbryio/lbrytv/app/publish"
	"github.com/lbryio/lbrytv/config"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoutesProxy(t *testing.T) {
	r := mux.NewRouter()
	proxy := proxy.NewService(config.GetLbrynet())

	req, err := http.NewRequest("POST", "/api/v1/proxy", bytes.NewBuffer([]byte(`{"method": "status"}`)))
	require.Nil(t, err)
	rr := httptest.NewRecorder()

	InstallRoutes(proxy, r)
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"result":`)
}

func TestRoutesPublish(t *testing.T) {
	r := mux.NewRouter()
	proxy := proxy.NewService(config.GetLbrynet())

	req := publish.CreatePublishRequest(t, []byte("test file"))
	rr := httptest.NewRecorder()

	InstallRoutes(proxy, r)
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	// Authentication Required error here is enough to see that the request
	// has been dispatched through the publish handler
	assert.Contains(t, rr.Body.String(), `"code": -32080`)
}

func TestRoutesOptions(t *testing.T) {
	r := mux.NewRouter()
	proxy := proxy.NewService(config.GetLbrynet())

	req, err := http.NewRequest("OPTIONS", "/api/v1/proxy", nil)
	require.Nil(t, err)
	rr := httptest.NewRecorder()

	InstallRoutes(proxy, r)
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "7200", rr.HeaderMap.Get("Access-Control-Max-Age"))
	assert.Equal(t, "*", rr.HeaderMap.Get("Access-Control-Allow-Origin"))
	assert.Equal(
		t,
		"X-Lbry-Auth-Token, Origin, X-Requested-With, Content-Type, Accept",
		rr.HeaderMap.Get("Access-Control-Allow-Headers"),
	)
}
