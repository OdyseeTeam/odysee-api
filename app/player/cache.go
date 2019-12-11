package player

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/dgraph-io/ristretto"
	"github.com/lbryio/lbrytv/internal/metrics"
)

const blobCost = 1 << 21     // 2MB
const maxCacheCost = 1 << 34 // 16GB
const blobFilenameLength = 96

// BlobCache can save and retrieve readable blobs.
type BlobCache interface {
	Has(string) bool
	Get(string) (ReadableBlob, bool)
	Set(string, []byte) (ReadableBlob, error)
	Remove(string)
}

type fsCache struct {
	storage *fsStorage
	rCache  *ristretto.Cache
}

type fsStorage struct {
	path string
}

type cachedBlob struct {
	reflectedBlob
}

// InitFSCache initializes disk cache for decrypted blobs.
// All blob-sized files inside `dir` will be removed on initialization,
// if `dir` does not exist, it will be created.
// In other words, os.TempDir() should not be passed as a `dir`.
func InitFSCache(dir string) (BlobCache, error) {
	storage, err := initFSStorage(dir)
	if err != nil {
		return nil, err
	}

	r, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7, // number of keys to track frequency of (10M)
		MaxCost:     maxCacheCost,
		BufferItems: 64,
		Metrics:     true,
		OnEvict:     func(_, _ uint64, hash interface{}, _ int64) { storage.remove(hash) },
	})
	if err != nil {
		return nil, err
	}

	return &fsCache{storage, r}, nil
}

func initFSStorage(dir string) (*fsStorage, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}

	// Cache folder cleanup
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if dir == path {
			return nil
		}
		if err != nil {
			return err
		}
		if info.IsDir() {
			return fmt.Errorf("subfolder %v found inside cache folder", path)
		}
		if len(info.Name()) != blobFilenameLength {
			return fmt.Errorf("non-cache file found at path %v", path)
		}
		return os.Remove(path)
	})

	if err != nil {
		return nil, err
	}
	return &fsStorage{dir}, nil
}

func (s fsStorage) remove(hash interface{}) {
	if err := os.Remove(s.getPath(hash)); err != nil {
		CacheLogger.Log().Errorf("failed to evict blob file %v: %v", hash, err)
		return
	}
	CacheLogger.Log().Debugf("blob file %v evicted", hash)
}

func (s fsStorage) getPath(hash interface{}) string {
	return path.Join(s.path, hash.(string))
}

func (s fsStorage) open(hash interface{}) (*os.File, error) {
	f, err := os.Open(s.getPath(hash))
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Has returns true if blob cache contains the requested blob.
// It is not guaranteed that actual file exists on the filesystem.
func (c *fsCache) Has(hash string) bool {
	_, ok := c.rCache.Get(hash)
	return ok
}

// Get returns ReadableBlob if it can be retrieved from the cache by the requested hash
// and a boolean representing whether blob was found or not.
func (c *fsCache) Get(hash string) (ReadableBlob, bool) {
	if value, ok := c.rCache.Get(hash); ok {
		f, err := c.storage.open(value)
		if err != nil {
			metrics.PlayerCacheErrorCount.Inc()
			CacheLogger.Log().Errorf("blob %v found in cache but couldn't open the file: %v", hash, err)
			c.rCache.Del(value)
			return nil, false
		}
		metrics.PlayerCacheHitCount.Inc()
		cb, err := initCachedBlob(f)
		f.Close()
		if err != nil {
			CacheLogger.Log().Errorf("blob %v found in cache but couldn't read the file: %v", hash, err)
			return nil, false
		}
		return cb, true
	}
	metrics.PlayerCacheMissCount.Inc()
	CacheLogger.Log().Debugf("cache miss for blob %v", hash)
	return nil, false
}

// Set takes decrypted blob body and saves reference to it into the cache table
func (c *fsCache) Set(hash string, body []byte) (ReadableBlob, error) {
	CacheLogger.Log().Debugf("attempting to cache blob %v", hash)
	blobPath := c.storage.getPath(hash)
	f, err := os.OpenFile(blobPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		metrics.PlayerCacheErrorCount.Inc()
		CacheLogger.Log().Debugf("blob %v already exists on the local filesystem, not overwriting", hash)
	} else {
		numWritten, err := f.Write(body)
		f.Close()
		if err != nil {
			metrics.PlayerCacheErrorCount.Inc()
			CacheLogger.Log().Errorf("error saving cache file %v: %v", blobPath, err)
			return nil, err
		}
		CacheLogger.Log().Debugf("written %v bytes for blob %v", numWritten, hash)
	}
	c.rCache.Set(hash, hash, blobCost)
	metrics.PlayerCacheSize.Set(float64(c.rCache.Metrics.CostAdded() - c.rCache.Metrics.CostEvicted()))
	CacheLogger.Log().Debugf("blob %v successfully cached", hash)
	return &cachedBlob{reflectedBlob{body}}, nil
}

// Remove deletes both cache record and blob file from the filesystem.
func (c *fsCache) Remove(hash string) {
	c.storage.remove(hash)
	c.rCache.Del(hash)
}

func initCachedBlob(file *os.File) (*cachedBlob, error) {
	body, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return &cachedBlob{reflectedBlob{body}}, nil
}
