package auth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMiddleware(t *testing.T) {
	r, err := http.NewRequest("GET", "/api/proxy", nil)
	require.NoError(t, err)
	r.Header.Set(wallet.TokenHeader, "secret-token")
	r.Header.Set("X-Forwarded-For", "8.8.8.8")

	var receivedRemoteIP string
	provider := func(token, ip string) Result {
		receivedRemoteIP = ip
		if token == "secret-token" {
			return NewResult(&models.User{ID: 16595}, nil)
		}
		return NewResult(nil, errors.New("error"))
	}

	rr := httptest.NewRecorder()
	Middleware(provider)(http.HandlerFunc(authChecker)).ServeHTTP(rr, r)

	response := rr.Result()
	body, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, "16595", string(body))
	assert.Equal(t, "8.8.8.8", receivedRemoteIP)
}

func TestMiddlewareAuthFailure(t *testing.T) {
	r, err := http.NewRequest("GET", "/api/proxy", nil)
	require.NoError(t, err)
	r.Header.Set(wallet.TokenHeader, "wrong-token")
	rr := httptest.NewRecorder()

	provider := func(token, ip string) Result {
		if token == "good-token" {
			return NewResult(&models.User{ID: 1}, nil)
		}
		return NewResult(nil, errors.New("incorrect token"))
	}
	Middleware(provider)(http.HandlerFunc(authChecker)).ServeHTTP(rr, r)

	response := rr.Result()
	body, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, "incorrect token", string(body))
	assert.Equal(t, http.StatusForbidden, response.StatusCode)
}

func TestMiddlewareNoAuth(t *testing.T) {
	r, err := http.NewRequest("GET", "/api/proxy", nil)
	require.NoError(t, err)
	rr := httptest.NewRecorder()

	provider := func(token, ip string) Result {
		if token == "good-token" {
			return NewResult(&models.User{ID: 1}, nil)
		}
		return NewResult(nil, errors.New("incorrect token"))
	}
	Middleware(provider)(http.HandlerFunc(authChecker)).ServeHTTP(rr, r)

	response := rr.Result()
	body, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
	assert.Equal(t, "no auth info", string(body))
}

func TestFromRequestSuccess(t *testing.T) {
	expected := NewResult(nil, errors.New("a test"))
	ctx := context.WithValue(context.Background(), ContextKey, expected)

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, "", &bytes.Buffer{})
	require.NoError(t, err)

	results := FromRequest(r)
	assert.NotNil(t, results)
	assert.Equal(t, expected.user, results.user)
	assert.Equal(t, expected.err.Error(), results.err.Error())
	assert.False(t, results.AuthAttempted())
}

func TestFromRequestFail(t *testing.T) {
	r, err := http.NewRequest(http.MethodPost, "", &bytes.Buffer{})
	require.NoError(t, err)
	assert.Panics(t, func() { FromRequest(r) })
}

func authChecker(w http.ResponseWriter, r *http.Request) {
	result := FromRequest(r)
	if result.user != nil && result.err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("this should never happen"))
		return
	}

	if !result.AuthAttempted() {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("no auth info"))
	} else if result.Authenticated() {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(fmt.Sprintf("%d", result.user.ID)))
	} else if result.Err() != nil {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(result.Err().Error()))
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("no user and no error. what happened?"))
	}
}
