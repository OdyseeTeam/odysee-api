package auth

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/internal/errors"
	"github.com/lbryio/lbrytv/internal/ip"
	"github.com/lbryio/lbrytv/internal/middleware"
	"github.com/lbryio/lbrytv/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/volatiletech/null"
)

func TestMiddleware_AuthSuccess(t *testing.T) {
	r, err := http.NewRequest("GET", "/api/proxy", nil)
	require.NoError(t, err)
	r.Header.Set(wallet.AuthorizationHeader, "secret-token")
	r.Header.Set("X-Forwarded-For", "8.8.8.8")

	var receivedRemoteIP string
	provider := func(token, ip string) (*models.User, error) {
		receivedRemoteIP = ip
		if token == "secret-token" {
			return &models.User{ID: 16595, IdpID: null.StringFrom("my-random-idp-id")}, nil
		}
		return nil, nil
	}

	rr := httptest.NewRecorder()
	middleware.Apply(middleware.Chain(
		ip.Middleware, Middleware(provider),
	), authChecker).ServeHTTP(rr, r)

	response := rr.Result()
	body, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, "16595", string(body))
	assert.Equal(t, "8.8.8.8", receivedRemoteIP)
}

func TestLegacyMiddleware_AuthSuccess(t *testing.T) {
	r, err := http.NewRequest("GET", "/api/proxy", nil)
	require.NoError(t, err)
	r.Header.Set(wallet.TokenHeader, "secret-token")
	r.Header.Set("X-Forwarded-For", "8.8.8.8")

	var receivedRemoteIP string
	provider := func(token, ip string) (*models.User, error) {
		receivedRemoteIP = ip
		if token == "secret-token" {
			return &models.User{ID: 16595}, nil
		}
		return nil, nil
	}

	rr := httptest.NewRecorder()
	middleware.Apply(middleware.Chain(
		ip.Middleware, LegacyMiddleware(provider),
	), authChecker).ServeHTTP(rr, r)

	response := rr.Result()
	body, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, "16595", string(body))
	assert.Equal(t, "8.8.8.8", receivedRemoteIP)
}

func TestLegacyMiddleware_AuthFailure(t *testing.T) {
	r, err := http.NewRequest("GET", "/api/proxy", nil)
	require.NoError(t, err)
	r.Header.Set(wallet.TokenHeader, "wrong-token")
	rr := httptest.NewRecorder()

	provider := func(token, ip string) (*models.User, error) {
		if token == "good-token" {
			return &models.User{ID: 1}, nil
		}
		return nil, nil
	}
	middleware.Apply(LegacyMiddleware(provider), authChecker).ServeHTTP(rr, r)

	response := rr.Result()
	body, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, "user not found", string(body))
	assert.Equal(t, http.StatusForbidden, response.StatusCode)
}

func TestLegacyMiddleware_NoAuthInfo(t *testing.T) {
	r, err := http.NewRequest("GET", "/api/proxy", nil)
	require.NoError(t, err)
	rr := httptest.NewRecorder()

	provider := func(token, ip string) (*models.User, error) {
		if token == "good-token" {
			return &models.User{ID: 1}, nil
		}
		return nil, nil
	}
	middleware.Apply(LegacyMiddleware(provider), authChecker).ServeHTTP(rr, r)

	response := rr.Result()
	body, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
	assert.Equal(t, "no auth info", string(body))
}

func TestLegacyMiddleware_Error(t *testing.T) {
	r, err := http.NewRequest("GET", "/api/proxy", nil)
	r.Header.Set(wallet.TokenHeader, "any-token")
	require.NoError(t, err)
	rr := httptest.NewRecorder()

	provider := func(token, ip string) (*models.User, error) {
		return nil, errors.Base("something broke")
	}
	middleware.Apply(LegacyMiddleware(provider), authChecker).ServeHTTP(rr, r)

	response := rr.Result()
	body, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	assert.Equal(t, "something broke", string(body))
}

func TestFromRequestSuccess(t *testing.T) {
	expected := result{nil, errors.Base("a test")}
	ctx := context.WithValue(context.Background(), contextKey, expected)

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, "", &bytes.Buffer{})
	require.NoError(t, err)

	var user *models.User
	assert.NotPanics(t, func() {
		user, err = FromRequest(r)
	})
	assert.Nil(t, user)
	assert.Equal(t, expected.err.Error(), err.Error())
}

func TestFromRequestFail(t *testing.T) {
	r, err := http.NewRequest(http.MethodPost, "", &bytes.Buffer{})
	require.NoError(t, err)
	user, err := FromRequest(r)
	assert.Nil(t, user)
	assert.Error(t, err)
	assert.Equal(t, "auth.Middleware is required", err.Error())
}

func authChecker(w http.ResponseWriter, r *http.Request) {
	user, err := FromRequest(r)
	if user != nil && err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("this should never happen"))
		return
	}

	if errors.Is(err, ErrNoAuthInfo) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("no auth info"))
	} else if user != nil {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(fmt.Sprintf("%d", user.ID)))
	} else if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
	} else {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("user not found"))
	}
}
