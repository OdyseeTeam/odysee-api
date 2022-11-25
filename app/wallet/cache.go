package wallet

import (
	"math/rand"
	"time"

	"github.com/OdyseeTeam/odysee-api/internal/metrics"
	"github.com/OdyseeTeam/odysee-api/internal/monitor"
	"github.com/OdyseeTeam/odysee-api/models"

	"github.com/dgraph-io/ristretto"
	"golang.org/x/sync/singleflight"
)

const (
	ttlUnconfirmed = 15 * time.Second
	ttlConfirmed   = 15 * time.Minute
)

var (
	cacheLogger  = monitor.NewModuleLogger("cache")
	currentCache *tokenCache
)

// tokenCache stores the cache in memory
type tokenCache struct {
	cache *ristretto.Cache
	sf    *singleflight.Group
}

func init() {
	SetTokenCache(NewTokenCache())
}

func NewTokenCache() *tokenCache {
	rc, _ := ristretto.NewCache(&ristretto.Config{
		MaxCost:     1 << 30,
		Metrics:     true,
		NumCounters: 1e7,
		BufferItems: 64,
	})
	return &tokenCache{
		cache: rc,
		sf:    &singleflight.Group{},
	}
}

func SetTokenCache(c *tokenCache) {
	currentCache = c
}

func (c *tokenCache) get(token string, retreiver func() (interface{}, error)) (*models.User, error) {
	var err error
	cachedUser, ok := c.cache.Get(token)
	if !ok {
		metrics.AuthTokenCacheMisses.Inc()
		cachedUser, err, _ = c.sf.Do(token, retreiver)
		if err != nil {
			return nil, err
		}
		var baseTTL time.Duration
		if cachedUser == nil {
			baseTTL = ttlUnconfirmed
		} else {
			baseTTL = ttlConfirmed
		}
		c.cache.SetWithTTL(token, cachedUser, 1, baseTTL+time.Duration(rand.Int63n(baseTTL.Nanoseconds())))
	} else {
		metrics.AuthTokenCacheHits.Inc()
	}

	if cachedUser == nil {
		return nil, nil
	}
	user := cachedUser.(*models.User)
	return user, nil
}

func (c *tokenCache) flush() {
	c.cache.Clear()
}
