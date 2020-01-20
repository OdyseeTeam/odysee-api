package proxy

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/lbryio/lbrytv/internal/monitor"

	"github.com/patrickmn/go-cache"
)

// CacheLogger is for logging query cache-related messages
var CacheLogger = monitor.NewModuleLogger("proxy_cache")

// ResponseCache interface describes methods for SDK response cache saving and retrieval
type ResponseCache interface {
	Save(method string, params interface{}, r interface{})
	Retrieve(method string, params interface{}) interface{}
	Count() int
	getKey(method string, params interface{}) (string, error)
	flush()
}

type cacheStorage struct {
	c *cache.Cache
}

var responseCache ResponseCache

// InitResponseCache initializes module-level responseCache variable
func InitResponseCache(c ResponseCache) {
	responseCache = c
}

// Save puts a response object into cache, making it available for a later retrieval by method and query params
func (s cacheStorage) Save(method string, params interface{}, r interface{}) {
	l := CacheLogger.LogF(monitor.F{"method": method})
	cacheKey, err := s.getKey(method, params)
	if err != nil {
		l.Errorf("unable to produce key for params: %v", params)
	} else {
		l.Info("saved query result")
	}
	s.c.Set(cacheKey, r, cache.DefaultExpiration)
}

// Retrieve earlier saved server response by method and query params
func (s cacheStorage) Retrieve(method string, params interface{}) interface{} {
	l := CacheLogger.LogF(monitor.F{"method": method})
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

func init() {
	InitResponseCache(cacheStorage{c: cache.New(2*time.Minute, 10*time.Minute)})
}
