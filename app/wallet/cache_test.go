package wallet

import (
	"errors"
	"testing"

	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/test"
	"github.com/lbryio/lbrytv/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/volatiletech/sqlboiler/boil"
)

func TestCache(t *testing.T) {
	setupTest()
	user, err := getOrCreateLocalUser(boil.GetDB(), models.User{ID: dummyUserID}, logger.Log())
	require.NoError(t, err)
	srv := test.RandServerAddress(t)
	rt := sdkrouter.New(map[string]string{"a": srv})
	err = assignSDKServerToUser(boil.GetDB(), user, rt.LeastLoaded(), logger.Log())
	require.NoError(t, err)

	cases := []struct {
		name, token   string
		retrievedUser *models.User
		err           error
		hitsInc       int
	}{
		{"UserNotFound", "nonexistingtoken", nil, errors.New("not found"), 0},
		{"ConfirmedUserFound", "confirmedtoken", user, nil, 1},
		{"UnconfirmedUserFound", "unconfirmedtoken", nil, nil, 1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hits := metrics.GetCounterValue(metrics.AuthTokenCacheHits)
			cachedUser, err := currentCache.get(c.token, func() (interface{}, error) {
				return c.retrievedUser, c.err
			})
			assert.Equal(t, c.retrievedUser, cachedUser)
			assert.Equal(t, c.err, err)
			currentCache.cache.Wait()
			cachedUser, err = currentCache.get(c.token, func() (interface{}, error) {
				return c.retrievedUser, c.err
			})
			assert.Equal(t, hits+float64(c.hitsInc), metrics.GetCounterValue(metrics.AuthTokenCacheHits))
			assert.Equal(t, c.retrievedUser, cachedUser)
			assert.Equal(t, c.err, err)
		})
	}
}
