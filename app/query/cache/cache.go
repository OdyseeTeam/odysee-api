package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/OdyseeTeam/odysee-api/internal/metrics"
	"github.com/OdyseeTeam/odysee-api/internal/monitor"
	"github.com/ybbus/jsonrpc"

	"github.com/dgraph-io/ristretto"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/singleflight"
)

type Retriever func() (interface{}, error)

type CacheConfig struct {
	size             int64
	ristrettoMetrics bool
}

// Cache manages SDK query responses.
type Cache struct {
	*CacheConfig
	cache *ristretto.Cache
	sf    *singleflight.Group
}

var cacheLogger = monitor.NewModuleLogger("cache")

func DefaultConfig() *CacheConfig {
	return &CacheConfig{
		size: 5 << 30, //  5GB
	}
}

func New(config *CacheConfig) (*Cache, error) {
	rc, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,
		MaxCost:     config.size,
		BufferItems: 64, // number of keys per Get buffer
		Metrics:     config.ristrettoMetrics,
	})
	if err != nil {
		return nil, err
	}
	c := Cache{
		CacheConfig: config,
		cache:       rc,
		sf:          &singleflight.Group{},
	}
	return &c, nil
}

func (c *CacheConfig) Size(size int64) *CacheConfig {
	c.size = size
	return c
}

// Retrieve earlier saved server response by method and query params.
func (c *Cache) Retrieve(method string, params interface{}, retriever Retriever) (interface{}, error) {
	k, err := c.hash(method, params)
	l := cacheLogger.WithFields(logrus.Fields{"key": k})

	if err != nil {
		l.Error("unable to produce cache key", "params", params, "err", err)
		return nil, err
	}
	res, ok := c.cache.Get(k)
	if !ok {
		metrics.ProxyQueryCacheMissCount.WithLabelValues(method).Inc()
		l.Debug("cache miss")
		if retriever == nil {
			return nil, errors.New("retriever is nil")
		}
		res, err, _ = c.sf.Do(k, retriever)
		if err != nil {
			l.Error("retriever failed", "err", err)
			return nil, err
		}

		resp, ok := res.(jsonrpc.RPCResponse)
		if ok && resp.Error != nil {
			l.Debug("rpc error reponse received, not caching")
			return res, nil
		}

		enc, err := json.Marshal(res)
		if err != nil {
			l.Error("failed to measure response size for cache", "err", err)
			return nil, err
		}
		l.WithFields(logrus.Fields{"size": len(enc)}).Debug("caching value")
		c.cache.SetWithTTL(k, res, int64(len(enc)), 3*time.Minute)
		return res, nil
	}
	metrics.ProxyQueryCacheHitCount.WithLabelValues(method).Inc()
	l.Debug("cache hit")
	return res, nil
}

func (c *Cache) hash(method string, params interface{}) (string, error) {
	if params == nil {
		return fmt.Sprintf("%v|nil", method), nil
	}
	h := sha256.New()
	enc, err := json.Marshal(params)
	if err != nil {
		return "", err
	}
	_, err = h.Write(enc)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%v|%v", method, hex.EncodeToString(h.Sum(nil))), nil
}

func (c *Cache) Flush() {
	c.cache.Clear()
}

func (c *Cache) Wait() {
	c.cache.Wait()
}

// count returns the number of non-expired items stored in cache.
func (c *Cache) count() uint64 {
	return c.cache.Metrics.KeysAdded() - c.cache.Metrics.KeysEvicted()
}
