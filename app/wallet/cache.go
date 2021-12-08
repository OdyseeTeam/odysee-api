package wallet

import (
	"time"

	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/models"

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
	SetTokenCache(NewTokenCache(10 * time.Minute))
}

func NewTokenCache(timeout time.Duration) *tokenCache {
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
		var ttl time.Duration
		if cachedUser == nil {
			ttl = ttlUnconfirmed
		} else {
			ttl = ttlConfirmed
		}
		c.cache.SetWithTTL(token, cachedUser, 1, ttl)
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
