package proxy

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/lbryio/lbrytv/internal/monitor"

	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
)

var (
	globalCache responseCache
	cacheLogger = monitor.NewModuleLogger("proxy_cache")
)

func init() {
	globalCache = cacheStorage{c: cache.New(2*time.Minute, 10*time.Minute)}
}

// responseCache interface describes methods for SDK response cache saving and retrieval
type responseCache interface {
	Save(method string, params interface{}, r interface{})
	Retrieve(method string, params interface{}) interface{}
	Count() int

	getKey(method string, params interface{}) (string, error)
	flush()
}

type cacheStorage struct {
	c *cache.Cache
}

// Save puts a response object into cache, making it available for a later retrieval by method and query params
func (s cacheStorage) Save(method string, params interface{}, r interface{}) {
	l := cacheLogger.WithFields(logrus.Fields{"method": method})
	cacheKey, err := s.getKey(method, params)
	if err != nil {
		l.Errorf("unable to produce key for params: %v", params)
	} else {
		l.Debug("saved query result")
	}
	s.c.Set(cacheKey, r, cache.DefaultExpiration)
}

// Retrieve earlier saved server response by method and query params
func (s cacheStorage) Retrieve(method string, params interface{}) interface{} {
	l := cacheLogger.WithFields(logrus.Fields{"method": method})
	cacheKey, err := s.getKey(method, params)
	if err != nil {
		l.Errorf("unable to produce key for params: %v", params)
		return nil
	}
	cachedResponse, ok := s.c.Get(cacheKey)
	if ok {
		l.Debug("query result found in cache")
	}
	return cachedResponse
}

func (s cacheStorage) getKey(method string, params interface{}) (key string, err error) {
	var paramsSuffix string

	if params != nil {
		h := sha256.New()
		enc := gob.NewEncoder(h)
		err = enc.Encode(fmt.Sprintf("%v", params))
		if err != nil {
			return "", err
		}
		paramsSuffix = hex.EncodeToString(h.Sum(nil))
	} else {
		paramsSuffix = "nil"
	}

	return fmt.Sprintf("%v|%v", method, paramsSuffix), err
}

func (s cacheStorage) flush() {
	s.c.Flush()
}

// Count returns the total number of non-expired items stored in cache
func (s cacheStorage) Count() int {
	return s.c.ItemCount()
}
