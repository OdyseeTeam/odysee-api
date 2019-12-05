package player

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/dgraph-io/ristretto"
)

const blobCost = 1 << 21     // 2MB
const maxCacheCost = 1 << 33 // 8GB

type BlobCache interface {
	Get(string) (ReadableBlob, bool)
	Set(string, []byte)
}

type fsCache struct {
	storage *fsStorage
	rCache  *ristretto.Cache
}

type fsStorage struct {
	path string
}

type cachedBlob struct {
	*os.File
}

func NewFSCache(dir string) (BlobCache, error) {
	storage, err := newFSStorage(dir)
	if err != nil {
		return nil, err
	}

	r, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7, // number of keys to track frequency of (10M)
		MaxCost:     maxCacheCost,
		BufferItems: 64,
		Metrics:     true,
		OnEvict:     storage.getBlobEvictor(),
	})
	if err != nil {
		return nil, err
	}

	return &fsCache{storage, r}, nil
}

func newFSStorage(dir string) (*fsStorage, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &fsStorage{dir}, nil
}

func (s fsStorage) getBlobEvictor() func(uint64, uint64, interface{}, int64) {
	return func(key, conflict uint64, value interface{}, cost int64) {
		Logger.Log().Debugf("blob %v evicted", value)
		if err := os.Remove(s.getBlobPath(value)); err != nil {
			fmt.Println(err)
		}
	}
}

func (s fsStorage) getBlobPath(hash interface{}) string {
	return path.Join(s.path, hash.(string))
}

func (s fsStorage) openBlob(hash interface{}) (*os.File, error) {
	f, err := os.Open(s.getBlobPath(hash))
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (c *fsCache) Get(hash string) (ReadableBlob, bool) {
	if value, ok := c.rCache.Get(hash); ok {
		f, err := c.storage.openBlob(value)
		if err != nil {
			Logger.Log().Warnf("blob %v found in cache but not on the local filesystem", hash)
			c.rCache.Del(value)
			return nil, false
		}
		return &cachedBlob{f}, true
	}
	Logger.Log().Warnf("cache miss for blob %v", hash)
	return nil, false
}

// Set takes decrypted blob body and saves reference to it into the cache table
func (c *fsCache) Set(hash string, body []byte) {
	blobPath := c.storage.getBlobPath(hash)
	f, err := os.OpenFile(blobPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		Logger.Log().Debugf("blob %v already exists on the local filesystem, not overwriting", hash)
	} else {
		numWritten, err := io.Copy(f, bytes.NewReader(body))
		if err != nil {
			Logger.Log().Errorf("error saving file %v: %v", f.Name(), err)
			return
		}
		Logger.Log().Debugf("written %v bytes for blob %v", numWritten, hash)
	}
	c.rCache.Set(hash, hash, blobCost)
	Logger.Log().Debugf("blob %v successfully cached", hash)
}

// Read reads the cached blob from the file
func (b *cachedBlob) Read(offset, n int, dest []byte) (int, error) {
	if n == -1 {
		stat, err := b.Stat()
		if err != nil {
			return 0, err
		}
		if int(stat.Size()) > len(dest) {
			n = len(dest)
		} else {
			n = int(stat.Size())
		}
	}
	// _, err := b.Seek(int64(offset), 0)
	// if err != nil {
	// 	return 0, err
	// }
	return b.ReadAt(dest[:n], int64(offset))
}
