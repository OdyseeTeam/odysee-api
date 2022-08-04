package auth

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OdyseeTeam/odysee-api/app/sdkrouter"
	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/errors"
	"github.com/OdyseeTeam/odysee-api/internal/ip"
	"github.com/OdyseeTeam/odysee-api/internal/middleware"
	"github.com/OdyseeTeam/odysee-api/internal/storage"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/migrator"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/volatiletech/null"
)

type dummyAuther struct {
	remoteIP string
}

func (a *dummyAuther) Authenticate(token, ip string) (*models.User, error) {
	a.remoteIP = ip
	if token != "secret-token" {
		return nil, fmt.Errorf("authentication failed")
	}
	return &models.User{ID: 16595, IdpID: null.StringFrom("my-random-idp-id")}, nil
}

func (a *dummyAuther) GetTokenFromHeader(_ http.Header) (string, error) {
	return "secret-token", nil
}

func dummyProvider(token, metaRemoteIP string) (*models.User, error) {
	if token == "secret-token" {
		return &models.User{ID: 16595, IdpID: null.StringFrom("my-random-idp-id")}, nil
	}
	return nil, nil
}

func TestMiddleware_AuthSuccess(t *testing.T) {
	r, err := http.NewRequest("GET", "/api/proxy", nil)
	require.NoError(t, err)
	r.Header.Set(wallet.LegacyTokenHeader, "secret-token")
	r.Header.Set("X-Forwarded-For", "8.8.8.8")

	auther := &dummyAuther{}

	rr := httptest.NewRecorder()
	middleware.Apply(middleware.Chain(
		ip.Middleware, Middleware(auther),
	), dummyHandler).ServeHTTP(rr, r)

	assert.Equal(t, "16595", rr.Body.String())
	assert.Equal(t, "8.8.8.8", rr.Result().Header.Get("x-remote-ip"))
}

func TestMiddleware_OAuthSuccess(t *testing.T) {
	db, dbCleanup, err := migrator.CreateTestDB(migrator.DBConfigFromApp(config.GetDatabase()), storage.MigrationsFS)
	require.NoError(t, err)
	storage.SetDB(db)
	defer dbCleanup()

	r, err := http.NewRequest("GET", "/api/proxy", nil)
	require.NoError(t, err)
	token, err := wallet.GetTestTokenHeader()
	require.NoError(t, err)

	r.Header.Set(wallet.AuthorizationHeader, token)
	r.Header.Set("X-Forwarded-For", "8.8.8.8")
	sdkRouter := sdkrouter.New(config.GetLbrynetServers())

	oauthAuther, err := wallet.NewOauthAuthenticator(
		config.GetOauthProviderURL(), config.GetOauthClientID(), config.GetInternalAPIHost(), sdkRouter)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	middleware.Apply(middleware.Chain(
		ip.Middleware, Middleware(oauthAuther), LegacyMiddleware(dummyProvider),
	), dummyHandler).ServeHTTP(rr, r)

	assert.Equal(t, "418533549", rr.Body.String())
	assert.Equal(t, "8.8.8.8", rr.Result().Header.Get("x-remote-ip"))
}

func TestLegacyMiddleware_AuthSuccess(t *testing.T) {
	r, err := http.NewRequest("GET", "/api/proxy", nil)
	require.NoError(t, err)
	r.Header.Set(wallet.LegacyTokenHeader, "secret-token")
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
	), dummyHandler).ServeHTTP(rr, r)

	assert.Equal(t, "16595", rr.Body.String())
	assert.Equal(t, "8.8.8.8", receivedRemoteIP)
}

func TestLegacyMiddleware_WrongToken(t *testing.T) {
	r, err := http.NewRequest("GET", "/api/proxy", nil)
	require.NoError(t, err)
	r.Header.Set(wallet.LegacyTokenHeader, "wrong-token")
	rr := httptest.NewRecorder()

	provider := func(token, _ string) (*models.User, error) {
		if token == "good-token" {
			return &models.User{ID: 1}, nil
		}
		return nil, nil
	}
	middleware.Apply(LegacyMiddleware(provider), dummyHandler).ServeHTTP(rr, r)

	assert.Equal(t, "user not found", rr.Body.String())
	assert.Equal(t, http.StatusForbidden, rr.Result().StatusCode)
}

func TestLegacyMiddleware_NoToken(t *testing.T) {
	r, err := http.NewRequest("GET", "/api/proxy", nil)
	require.NoError(t, err)
	rr := httptest.NewRecorder()

	provider := func(token, _ string) (*models.User, error) {
		if token == "good-token" {
			return &models.User{ID: 1}, nil
		}
		return nil, nil
	}
	middleware.Apply(LegacyMiddleware(provider), dummyHandler).ServeHTTP(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Result().StatusCode)
	assert.Equal(t, "no auth info", rr.Body.String())
}

func TestLegacyMiddleware_Error(t *testing.T) {
	r, err := http.NewRequest("GET", "/api/proxy", nil)
	r.Header.Set(wallet.LegacyTokenHeader, "any-token")
	require.NoError(t, err)
	rr := httptest.NewRecorder()

	provider := func(token, ip string) (*models.User, error) {
		return nil, errors.Base("something broke")
	}
	middleware.Apply(LegacyMiddleware(provider), dummyHandler).ServeHTTP(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Result().StatusCode)
	assert.Equal(t, "something broke", rr.Body.String())
}

func TestFromRequestSuccess(t *testing.T) {
	expected := &CurrentUser{user: nil, err: errors.Base("a test")}
	ctx := context.WithValue(context.Background(), userContextKey, expected)

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
	assert.Equal(t, "auth middleware is required", err.Error())
}

func dummyHandler(w http.ResponseWriter, r *http.Request) {
	cu, err := GetCurrentUserData(r.Context())
	w.Header().Add("x-remote-ip", cu.IP)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("unexpected error: %s", err)))
		return
	}
	user := cu.user
	err = cu.err
	if user != nil && err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("both user and error is set"))
		return
	}

	if errors.Is(err, wallet.ErrNoAuthInfo) {
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
