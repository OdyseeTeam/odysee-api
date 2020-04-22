package monitor

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	log "github.com/sirupsen/logrus"
	logrusTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

type middlewareTestRow struct {
	NAME                  string
	reqBody               *io.Reader
	handler               http.HandlerFunc
	expectedStatus        int
	expectedBody          string
	expectedJSONMessage   string
	expectedLogEntry      map[string]interface{}
	expectedLogLevel      log.Level
	expectedLogStartsWith string
}

var testTableErrorLoggingMiddleware = []middlewareTestRow{
	{
		NAME: "Panicking Handler",
		handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`everything good so far...`))
			panic("panic ensued")
		},
		expectedStatus:      http.StatusInternalServerError,
		expectedJSONMessage: "panic ensued",
		expectedLogEntry: map[string]interface{}{
			"method":   "POST",
			"url":      "/some-endpoint",
			"status":   http.StatusAccepted, // this is the original status before the panic
			"response": "everything good so far...",
		},
		expectedLogLevel:      log.ErrorLevel,
		expectedLogStartsWith: "RECOVERED PANIC: panic ensued",
	},

	{
		NAME: "Erroring Handler",
		handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"status": "error"}`))
		},
		expectedStatus: http.StatusBadRequest,
		expectedBody:   `{"status": "error"}`,
		expectedLogEntry: map[string]interface{}{
			"method":   "POST",
			"url":      "/some-endpoint",
			"status":   http.StatusBadRequest,
			"response": `{"status": "error"}`,
		},
		expectedLogLevel:      log.ErrorLevel,
		expectedLogStartsWith: "handler responded with an error",
	},

	{
		NAME: "Redirecting Handler",
		handler: func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://lbry.com", http.StatusPermanentRedirect)
		},
		expectedStatus: http.StatusPermanentRedirect,
	},

	{
		NAME: "Okay Handler",
		handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`OK`))
		},
		expectedStatus: http.StatusAccepted,
		expectedBody:   "OK",
	},

	{
		NAME: "Panic Recovered Handler",
		handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			defer func() {
				if r := recover(); r != nil {
					w.Write([]byte("recovered in handler"))
				}
			}()
			panic("xoxoxo")
		},
		expectedStatus: http.StatusAccepted,
		expectedBody:   "recovered in handler",
	},
}

func TestErrorLoggingMiddlewareTableTest(t *testing.T) {
	for _, row := range testTableErrorLoggingMiddleware {
		t.Run(row.NAME, func(t *testing.T) {
			hook := logrusTest.NewLocal(httpLogger.Entry.Logger)

			var reqBody io.Reader
			if row.reqBody != nil {
				reqBody = *row.reqBody
			}
			r, err := http.NewRequest("POST", "/some-endpoint", reqBody)
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			mw := ErrorLoggingMiddleware(row.handler)
			assert.NotPanics(t, func() {
				mw.ServeHTTP(rr, r)
			})
			res := rr.Result()
			body, err := ioutil.ReadAll(res.Body)
			require.NoError(t, err)

			assert.Equal(t, row.expectedStatus, res.StatusCode)
			if row.expectedBody != "" {
				assert.Equal(t, row.expectedBody, string(body))
			} else if row.expectedJSONMessage != "" {
				var decoded jsonrpc.RPCResponse
				err := json.Unmarshal(body, &decoded)
				require.NoError(t, err)
				assert.Equal(t, row.expectedJSONMessage, decoded.Error.Message)
			} else {
				assert.Equal(t, "", string(body))
			}

			if row.expectedLogEntry != nil {
				if assert.GreaterOrEqual(t, len(hook.Entries), 1, "expected a log entry, got none") {
					l := hook.LastEntry()
					assert.Equal(t, row.expectedLogLevel, l.Level)
					for k, v := range row.expectedLogEntry {
						assert.Equal(t, v, l.Data[k], k)
					}

					assert.Regexp(t, "^"+row.expectedLogStartsWith, l.Message)
				}
			}
		})
	}
}
