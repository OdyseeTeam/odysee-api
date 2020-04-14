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
	retriever := func(token, ip string) (*models.User, error) {
		receivedRemoteIP = ip
		if token == "secret-token" {
			return &models.User{ID: 16595}, nil
		}
		return nil, errors.New("error")
	}

	rr := httptest.NewRecorder()
	Middleware(retriever)(http.HandlerFunc(authChecker)).ServeHTTP(rr, r)

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

	retriever := func(token, ip string) (*models.User, error) {
		if token == "good-token" {
			return &models.User{ID: 1}, nil
		}
		return nil, errors.New("incorrect token")
	}
	Middleware(retriever)(http.HandlerFunc(authChecker)).ServeHTTP(rr, r)

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

	retriever := func(token, ip string) (*models.User, error) {
		if token == "good-token" {
			return &models.User{ID: 1}, nil
		}
		return nil, errors.New("incorrect token")
	}
	Middleware(retriever)(http.HandlerFunc(authChecker)).ServeHTTP(rr, r)

	response := rr.Result()
	body, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
	assert.Equal(t, "no auth info", string(body))
}

func TestFromRequestSuccess(t *testing.T) {
	expected := &Result{Err: errors.New("a test")}
	ctx := context.WithValue(context.Background(), ContextKey, expected)

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, "", &bytes.Buffer{})
	require.NoError(t, err)

	results := FromRequest(r)
	assert.NotNil(t, results)
	assert.Equal(t, expected.User, results.User)
	assert.Equal(t, expected.Err.Error(), results.Err.Error())
}

func TestFromRequestFail(t *testing.T) {
	r, err := http.NewRequest(http.MethodPost, "", &bytes.Buffer{})
	require.NoError(t, err)
	assert.Panics(t, func() { FromRequest(r) })
}

func authChecker(w http.ResponseWriter, r *http.Request) {
	result := FromRequest(r)
	if result.Authenticated() {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(fmt.Sprintf("%d", result.User.ID)))
	} else if result.AuthFailed() {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(result.Err.Error()))
	} else {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("no auth info"))
	}
}
