package wallet

import (
	"time"

	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/models"

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
	SetTokenCache(NewTokenCache(10 * time.Minute))
}

func NewTokenCache(timeout time.Duration) *tokenCache {
	return &tokenCache{cache: gocache.New(timeout, 45*time.Minute)}
}

func SetTokenCache(c *tokenCache) {
	currentCache = c
}

func (c *tokenCache) set(token string, user *models.User) {
	c.cache.Set(token, *user, gocache.DefaultExpiration)
}

func (c *tokenCache) get(token string) *models.User {
	obj, ok := c.cache.Get(token)
	if !ok {
		metrics.AuthTokenCacheMisses.Inc()
		return nil
	}
	metrics.AuthTokenCacheHits.Inc()
	user := obj.(models.User)
	return &user
}

func (c *tokenCache) flush() {
	c.cache.Flush()
}
