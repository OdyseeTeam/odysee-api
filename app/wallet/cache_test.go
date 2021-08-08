package wallet

import (
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

	metricValue := metrics.GetCounterValue(metrics.AuthTokenCacheHits)

	token := "abc"
	cachedUser := currentCache.get(token)
	assert.Nil(t, cachedUser)
	assert.Equal(t, metricValue, metrics.GetCounterValue(metrics.AuthTokenCacheHits))

	user, err := getOrCreateLocalUser(boil.GetDB(), models.User{ID: dummyUserID}, logger.Log())
	require.NoError(t, err)

	srv := test.RandServerAddress(t)
	rt := sdkrouter.New(map[string]string{"a": srv})
	// url, cleanup := dummyAPI(srv)
	// defer cleanup()

	err = assignSDKServerToUser(boil.GetDB(), user, rt.LeastLoaded(), logger.Log())
	require.NoError(t, err)

	currentCache.set(token, user)
	cachedUser = currentCache.get(token)
	assert.Equal(t, metricValue+1, metrics.GetCounterValue(metrics.AuthTokenCacheHits))
	assert.Equal(t, user.LbrynetServerID, cachedUser.LbrynetServerID)
	cachedUser.ID = 111111
	cachedUser = currentCache.get(token)
	assert.Equal(t, user.ID, cachedUser.ID)
}
