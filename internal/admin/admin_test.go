package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OdyseeTeam/odysee-api/pkg/iprate"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func TestSimpleAdminAuthMiddleware(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	limiter := iprate.NewLimiter(rate.Limit(0.05), 3)
	middleware := SimpleAdminAuthMiddleware("test-token", limiter)(nextHandler)

	t.Run("returns 500 when token not configured", func(t *testing.T) {
		middleware := SimpleAdminAuthMiddleware("", limiter)(nextHandler)

		req := httptest.NewRequest("GET", "/admin", nil)
		recorder := httptest.NewRecorder()

		middleware.ServeHTTP(recorder, req)

		require.Equal(t, http.StatusInternalServerError, recorder.Code)
	})

	t.Run("returns 403 with incorrect token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin", nil)
		req.Header.Set("Authorization", "Bearer wrong-token")
		recorder := httptest.NewRecorder()

		middleware.ServeHTTP(recorder, req)

		require.Equal(t, http.StatusUnauthorized, recorder.Code)
	})

	t.Run("returns 429 after too many attempts", func(t *testing.T) {
		originalLimiter := limiter
		defer func() { limiter = originalLimiter }()

		limiter := iprate.NewLimiter(rate.Limit(0.1), 1)
		middleware := SimpleAdminAuthMiddleware("test-token", limiter)(nextHandler)

		req1 := httptest.NewRequest("GET", "/admin", nil)
		req1.Header.Set("Authorization", "Bearer wrong-token")
		req1.RemoteAddr = "192.168.1.100:12345"
		recorder1 := httptest.NewRecorder()

		middleware.ServeHTTP(recorder1, req1)

		require.Equal(t, http.StatusUnauthorized, recorder1.Code)

		req2 := httptest.NewRequest("GET", "/admin", nil)
		req2.Header.Set("Authorization", "Bearer wrong-token")
		req2.RemoteAddr = "192.168.1.100:12345"
		recorder2 := httptest.NewRecorder()

		middleware.ServeHTTP(recorder2, req2)

		require.Equal(t, http.StatusTooManyRequests, recorder2.Code)
	})

	t.Run("allows request with correct token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		recorder := httptest.NewRecorder()

		middleware.ServeHTTP(recorder, req)

		require.Equal(t, http.StatusOK, recorder.Code)
		require.Equal(t, "success", recorder.Body.String())
	})

	t.Run("different IPs are rate limited separately", func(t *testing.T) {
		limiter := iprate.NewLimiter(rate.Limit(0.1), 1)
		middleware := SimpleAdminAuthMiddleware("test-token", limiter)(nextHandler)

		req1 := httptest.NewRequest("GET", "/admin", nil)
		req1.Header.Set("Authorization", "Bearer wrong-token")
		req1.RemoteAddr = "10.0.0.1:12345"
		recorder1 := httptest.NewRecorder()

		middleware.ServeHTTP(recorder1, req1)

		require.Equal(t, http.StatusUnauthorized, recorder1.Code)

		req2 := httptest.NewRequest("GET", "/admin", nil)
		req2.Header.Set("Authorization", "Bearer wrong-token")
		req2.RemoteAddr = "10.0.0.2:12345"
		recorder2 := httptest.NewRecorder()

		middleware.ServeHTTP(recorder2, req2)

		require.Equal(t, http.StatusUnauthorized, recorder2.Code)

		req3 := httptest.NewRequest("GET", "/admin", nil)
		req3.Header.Set("Authorization", "Bearer wrong-token")
		req3.RemoteAddr = "10.0.0.1:12346"
		recorder3 := httptest.NewRecorder()

		middleware.ServeHTTP(recorder3, req3)

		require.Equal(t, http.StatusTooManyRequests, recorder3.Code)
	})
}

func Test_sdkDetails(t *testing.T) {
	d, err := parseSDKDetails("lbrynet-a-2")
	require.NoError(t, err)
	assert.Equal(t, d.id, 2)
	assert.Equal(t, d.group, "a")
	d.bumpID()
	assert.Equal(t, d.id, 3)
	assert.Equal(t, d.String(), "lbrynet-a-3")
}
