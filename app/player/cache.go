package player

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/dgraph-io/ristretto"
	"github.com/lbryio/lbry.go/v2/stream"
	"github.com/lbryio/lbrytv/internal/metrics"
)

const defaultMaxCacheSize = 1 << 34 // 16GB

// BlobCache can save and retrieve readable chunks.
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

// FSCacheOpts contains options for filesystem cache. Size is max size in bytes
type FSCacheOpts struct {
	Path string
	Size int64
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
func InitFSCache(opts *FSCacheOpts) (BlobCache, error) {
	storage, err := initFSStorage(opts.Path)
	if err != nil {
		return nil, err
	}

	if opts.Size == 0 {
		opts.Size = defaultMaxCacheSize
	}

	r, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7, // number of keys to track frequency of (10M)
		MaxCost:     opts.Size,
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

	// Cache folder cleanup performed based on file names, chunk files will have a name of certain length.
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
		if len(info.Name()) != stream.BlobHashHexLength {
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
	var size int64
	fi, _ := os.Stat(s.getPath(hash))
	if fi != nil {
		size = fi.Size()
	}

	if err := os.Remove(s.getPath(hash)); err != nil {
		CacheLogger.Log().Errorf("failed to evict blob file %v: %v", hash, err)
		return
	}
	metrics.PlayerCacheSize.Sub(float64(size))
	CacheLogger.Log().Infof("blob file %v evicted", hash)
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
	cacheCost := len(body)

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
	c.rCache.Set(hash, hash, int64(cacheCost))
	// metrics.PlayerCacheSize.Set(float64(c.rCache.Metrics.CostAdded() - c.rCache.Metrics.CostEvicted()))

	metrics.PlayerCacheSize.Add(float64(cacheCost))
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
