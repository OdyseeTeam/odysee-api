package server

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	log "github.com/sirupsen/logrus"
	logrus_test "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type middlewareTestRow struct {
	NAME             string
	method           string
	url              string
	reqBody          *io.Reader
	handler          http.HandlerFunc
	status           int
	resBody          string
	lastEntry        *map[string]interface{}
	lastEntryLevel   log.Level
	lastEntryMessage string
}

var testTableErrorLoggingMiddleware = []middlewareTestRow{
	middlewareTestRow{
		NAME:   "Panicking Handler",
		method: "POST",
		url:    "/api/",
		handler: func(w http.ResponseWriter, r *http.Request) {
			panic("panic ensued")
		},
		status:  http.StatusInternalServerError,
		resBody: "panic ensued\n",
		lastEntry: &map[string]interface{}{
			"method":   "POST",
			"url":      "/api/",
			"status":   http.StatusInternalServerError,
			"response": "panic ensued",
		},
		lastEntryLevel:   log.ErrorLevel,
		lastEntryMessage: "panic ensued",
	},

	middlewareTestRow{
		NAME:   "Erroring Handler",
		method: "POST",
		url:    "/api/",
		handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"status": "error"}`))
		},
		status:  http.StatusBadRequest,
		resBody: `{"status": "error"}`,
		lastEntry: &map[string]interface{}{
			"method":   "POST",
			"url":      "/api/",
			"status":   http.StatusBadRequest,
			"response": `{"status": "error"}`,
		},
		lastEntryLevel:   log.ErrorLevel,
		lastEntryMessage: "handler responded with an error",
	},

	middlewareTestRow{
		NAME:   "Redirecting Handler",
		method: "POST",
		url:    "/api/",
		handler: func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://lbry.com", http.StatusPermanentRedirect)
		},
		status: http.StatusPermanentRedirect,
	},

	middlewareTestRow{
		NAME:   "Okay Handler",
		method: "POST",
		url:    "/api/",
		handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`OK`))
		},
		status:  http.StatusAccepted,
		resBody: "OK",
	},
}

func TestErrorLoggingMiddlewareTableTest(t *testing.T) {
	for _, row := range testTableErrorLoggingMiddleware {
		t.Run(row.NAME, func(t *testing.T) {
			hook := logrus_test.NewLocal(logger.Logger)

			var reqBody io.Reader
			if row.reqBody != nil {
				reqBody = *row.reqBody
			}
			r, _ := http.NewRequest(row.method, row.url, reqBody)

			mw := ErrorLoggingMiddleware(http.HandlerFunc(row.handler))
			rr := httptest.NewRecorder()
			mw.ServeHTTP(rr, r)
			res := rr.Result()
			body, err := ioutil.ReadAll(res.Body)
			require.Nil(t, err)

			assert.Equal(t, row.status, res.StatusCode)
			assert.Equal(t, row.resBody, string(body))

			if row.lastEntry != nil {
				if assert.GreaterOrEqual(t, len(hook.Entries), 1, "expected a log entry, got none") {
					l := hook.LastEntry()
					assert.Equal(t, row.lastEntryLevel, l.Level)
					for k, v := range *row.lastEntry {
						assert.Equal(t, v, l.Data[k], k)
					}
					assert.Equal(t, row.lastEntryMessage, l.Message)
				}
			}
		})
	}
}
