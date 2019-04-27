package proxy

import (
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"time"

	"github.com/patrickmn/go-cache"
)

// ResponseCache interface describes methods for SDK response cache saving and retrieval
type ResponseCache interface {
	Save(method string, params interface{}, r interface{}) error
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
func (s *cacheStorage) Save(method string, params interface{}, r interface{}) error {
	cacheKey, err := s.getKey(method, params)
	if err != nil {
		panic("unable to get key")
	}
	s.c.Set(cacheKey, r, cache.DefaultExpiration)
	return nil
}

// Retrieve earlier saved server response by method and query params
func (s *cacheStorage) Retrieve(method string, params interface{}) (cachedResponse interface{}) {
	cacheKey, err := s.getKey(method, params)
	if err != nil {
		panic("unable to get key")
	}
	cachedResponse, _ = s.c.Get(cacheKey)
	return cachedResponse
}

func (s *cacheStorage) getKey(method string, params interface{}) (key string, err error) {
	h := sha256.New()
	paramsMap := params.(map[string]interface{})
	gob.Register(paramsMap)
	for _, v := range paramsMap {
		gob.Register(v)
	}
	enc := gob.NewEncoder(h)
	err = enc.Encode(params)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%v|%v", method, h.Sum(nil)), err
}

func (s *cacheStorage) flush() {
	s.c.Flush()
}

// Count returns the total number of non-expired items stored in cache
func (s *cacheStorage) Count() int {
	return s.c.ItemCount()
}

func init() {
	InitResponseCache(&cacheStorage{c: cache.New(2*time.Minute, 10*time.Minute)})
}
