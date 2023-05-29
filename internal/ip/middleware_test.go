package ip

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OdyseeTeam/odysee-api/internal/middleware"

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
