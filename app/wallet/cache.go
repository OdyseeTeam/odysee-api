package wallet

import (
	"time"

	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"

	gocache "github.com/patrickmn/go-cache"
)

var (
	cacheLogger  = monitor.NewModuleLogger("cache")
	currentCache *tokenCache
)

// tokenCache stores the cache in memory
type tokenCache struct {
	cache *gocache.Cache
}

func init() {
	SetTokenCache(NewTokenCache(5 * time.Minute))
}

func NewTokenCache(timeout time.Duration) *tokenCache {
	return &tokenCache{cache: gocache.New(timeout, 10*time.Minute)}
}

func SetTokenCache(c *tokenCache) {
	currentCache = c
}

func (c *tokenCache) set(token string, userID int) {
	c.cache.Set(token, userID, gocache.DefaultExpiration)
}

func (c *tokenCache) get(token string) int {
	uid, ok := c.cache.Get(token)
	if !ok {
		metrics.AuthTokenCacheMisses.Inc()
		return 0
	}
	metrics.AuthTokenCacheHits.Inc()
	return uid.(int)
}
