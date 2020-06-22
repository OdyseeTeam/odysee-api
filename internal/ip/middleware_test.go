package ip

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lbryio/lbrytv/internal/middleware"

	logrusTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func TestMiddleware(t *testing.T) {
	for val, exp := range expectedIPs {
		t.Run(val, func(t *testing.T) {
			r, _ := http.NewRequest(http.MethodGet, "", nil)
			r.Header.Add("X-Forwarded-For", val)

			rr := httptest.NewRecorder()
			mw := middleware.Apply(Middleware, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, exp, FromRequest(r))
			})
			mw.ServeHTTP(rr, r)
		})
	}
}

func TestFromRequest_MiddlewareNotApplied(t *testing.T) {
	logHook := logrusTest.NewLocal(logger.Entry.Logger)

	r, _ := http.NewRequest(http.MethodGet, "", nil)
	r.Header.Add("X-Forwarded-For", "8.8.8.8")
	rr := httptest.NewRecorder()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "", FromRequest(r))
		assert.Contains(t, logHook.LastEntry().Message, "ip.Middleware wasn't applied")
	})
	h.ServeHTTP(rr, r)
}
