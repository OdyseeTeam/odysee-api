package server

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func panickingHandler(w http.ResponseWriter, r *http.Request) {
	panic("a plain panic happened")
}

func erroringHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(`{"status": "error"}`))
}

func okayHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`OK`))
}

func TestErrorLoggingMiddlewarePanic(t *testing.T) {
	r, _ := http.NewRequest("POST", "/api/", nil)
	hook := test.NewLocal(logger.Logger)

	panicMW := ErrorLoggingMiddleware(http.HandlerFunc(panickingHandler))

	panicRR := httptest.NewRecorder()
	panicMW.ServeHTTP(panicRR, r)

	res := panicRR.Result()
	body, err := ioutil.ReadAll(res.Body)
	require.Nil(t, err)

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	assert.Equal(t, "a plain panic happened\n", string(body))

	lastLog := hook.LastEntry()
	require.Equal(t, 1, len(hook.Entries))
	require.Equal(t, log.ErrorLevel, lastLog.Level)
	require.Equal(t, "POST", lastLog.Data["method"])
	require.Equal(t, "/api/", lastLog.Data["url"])
	require.Equal(t, "500", lastLog.Data["status"])
	require.Equal(t, `{"status": "error"}`, lastLog.Data["response"])
	require.Equal(t, `handler responded with an error`, lastLog.Message)
}

func TestErrorLoggingMiddlewareError(t *testing.T) {
	r, _ := http.NewRequest("POST", "/api/", nil)
	hook := test.NewLocal(logger.Logger)

	errorMW := ErrorLoggingMiddleware(http.HandlerFunc(erroringHandler))

	errorRR := httptest.NewRecorder()
	errorMW.ServeHTTP(errorRR, r)

	res := errorRR.Result()
	body, err := ioutil.ReadAll(res.Body)
	require.Nil(t, err)

	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	assert.Equal(t, `{"status": "error"}`, string(body))

	require.Equal(t, 1, len(hook.Entries))
	lastLog := hook.LastEntry()
	assert.Equal(t, log.ErrorLevel, lastLog.Level)
	assert.Equal(t, "POST", lastLog.Data["method"])
	assert.Equal(t, "/api/", lastLog.Data["url"])
	assert.Equal(t, "401", lastLog.Data["status"])
	assert.Equal(t, "a plain panic happened", lastLog.Data["response"])
	assert.Equal(t, `handler responded with an error`, lastLog.Message)
}

func TestErrorLoggingMiddlewareOK(t *testing.T) {
	r, _ := http.NewRequest("POST", "/api/", nil)
	hook := test.NewLocal(logger.Logger)

	okMW := ErrorLoggingMiddleware(http.HandlerFunc(okayHandler))

	okRR := httptest.NewRecorder()
	okMW.ServeHTTP(okRR, r)

	res := okRR.Result()
	body, err := ioutil.ReadAll(res.Body)
	require.Nil(t, err)

	assert.Equal(t, http.StatusAccepted, res.StatusCode)
	assert.Equal(t, `OK`, string(body))

	require.Equal(t, 0, len(hook.Entries))
}
