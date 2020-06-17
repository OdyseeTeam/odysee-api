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
	"github.com/lbryio/lbrytv/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMiddleware_AuthSuccess(t *testing.T) {
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
	Middleware(provider)(http.HandlerFunc(authChecker)).ServeHTTP(rr, r)

	response := rr.Result()
	body, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, "16595", string(body))
	assert.Equal(t, "8.8.8.8", receivedRemoteIP)
}

func TestMiddleware_AuthFailure(t *testing.T) {
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
	Middleware(provider)(http.HandlerFunc(authChecker)).ServeHTTP(rr, r)

	response := rr.Result()
	body, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, "user not found", string(body))
	assert.Equal(t, http.StatusForbidden, response.StatusCode)
}

func TestMiddleware_NoAuthInfo(t *testing.T) {
	r, err := http.NewRequest("GET", "/api/proxy", nil)
	require.NoError(t, err)
	rr := httptest.NewRecorder()

	provider := func(token, ip string) (*models.User, error) {
		if token == "good-token" {
			return &models.User{ID: 1}, nil
		}
		return nil, nil
	}
	Middleware(provider)(http.HandlerFunc(authChecker)).ServeHTTP(rr, r)

	response := rr.Result()
	body, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
	assert.Equal(t, "no auth info", string(body))
}

func TestMiddleware_Error(t *testing.T) {
	r, err := http.NewRequest("GET", "/api/proxy", nil)
	r.Header.Set(wallet.TokenHeader, "any-token")
	require.NoError(t, err)
	rr := httptest.NewRecorder()

	provider := func(token, ip string) (*models.User, error) {
		return nil, errors.Base("something broke")
	}
	Middleware(provider)(http.HandlerFunc(authChecker)).ServeHTTP(rr, r)

	response := rr.Result()
	body, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	assert.Equal(t, "something broke", string(body))
}

func TestFromRequestReturnsResult(t *testing.T) {
	expected := Result{User: nil, RemoteIP: "8.8.8.8", err: errors.Base("some imaginary error")}
	ctx := context.WithValue(context.Background(), contextKey, expected)

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, "", &bytes.Buffer{})
	require.NoError(t, err)

	var res Result
	assert.NotPanics(t, func() {
		res, err = FromRequest(r)
	})
	assert.Nil(t, res.User)
	assert.Equal(t, expected.err.Error(), err.Error())
	assert.Equal(t, "8.8.8.8", res.RemoteIP)
}

func TestFromRequestFail(t *testing.T) {
	r, err := http.NewRequest(http.MethodPost, "", &bytes.Buffer{})
	require.NoError(t, err)
	res, err := FromRequest(r)
	assert.Nil(t, res.User)
	assert.Error(t, err)
	assert.Equal(t, "auth.Middleware is required", err.Error())
}

func authChecker(w http.ResponseWriter, r *http.Request) {
	res, err := FromRequest(r)
	if res.User != nil && err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("this should never happen"))
		return
	}

	if errors.Is(err, ErrNoAuthInfo) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("no auth info"))
	} else if res.User != nil {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(fmt.Sprintf("%d", res.User.ID)))
	} else if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
	} else {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("user not found"))
	}
}
