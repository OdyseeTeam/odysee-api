package player

import (
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/lbryio/lbry.go/v2/stream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func waitForCache() {
	time.Sleep(time.Millisecond * 10)
}

func generateCachePath() string {
	return path.Join(os.TempDir(), randomString(50))
}

func TestFSCache(t *testing.T) {
	dir := generateCachePath()
	os.RemoveAll(dir)

	_, err := InitFSCache(&FSCacheOpts{Path: dir})
	require.Nil(t, err)

	fi, err := os.Stat(dir)
	require.NoError(t, err)
	assert.Equal(t, "drwx------", fi.Mode().String())

	os.Remove(dir)
}

func TestFSCacheClearsFolder(t *testing.T) {
	dir := generateCachePath()
	os.MkdirAll(dir, 0700)

	defer os.RemoveAll(dir)

	fileToBeRemoved := path.Join(dir, randomString(stream.BlobHashHexLength))
	f, err := os.Create(fileToBeRemoved)
	require.NoError(t, err)
	n, err := f.Write(make([]byte, stream.MaxBlobSize))
	require.NoError(t, err)
	require.Equal(t, stream.MaxBlobSize, n)
	f.Close()

	_, err = InitFSCache(&FSCacheOpts{Path: dir})
	require.Nil(t, err)

	_, err = os.Stat(fileToBeRemoved)
	assert.Error(t, err)

	fileToNotBeRemoved := path.Join(dir, "non_blob_sized_file_name")
	f, err = os.Create(fileToNotBeRemoved)
	require.NoError(t, err)

	// Cleanup
	defer os.Remove(fileToNotBeRemoved)

	n, err = f.Write(make([]byte, stream.MaxBlobSize/2))
	require.NoError(t, err)
	require.Equal(t, stream.MaxBlobSize/2, n)
	f.Close()

	_, err = InitFSCache(&FSCacheOpts{Path: dir})
	require.Error(t, err)
}

func TestFSCacheHas(t *testing.T) {
	c, err := InitFSCache(&FSCacheOpts{Path: generateCachePath()})
	require.NoError(t, err)

	assert.False(t, c.Has("hAsH"))
	c.Set("hAsH", []byte{1, 2, 3})

	waitForCache()
	assert.True(t, c.Has("hAsH"))

	c.Remove("hAsH")
	waitForCache()
	assert.False(t, c.Has("hAsH"))
}

func TestFSCacheSetGet(t *testing.T) {
	c, err := InitFSCache(&FSCacheOpts{Path: generateCachePath()})
	require.NoError(t, err)

	b, ok := c.Get("hAsH")
	assert.Nil(t, b)
	assert.False(t, ok)

	c.Set("hAsH", []byte{1, 2, 3})
	defer c.Remove("hAsH")

	waitForCache()
	b, ok = c.Get("hAsH")
	require.True(t, ok)

	read := make([]byte, 3)
	b.Read(0, 3, read)
	assert.Equal(t, []byte{1, 2, 3}, read)
}

func TestFSCacheRemove(t *testing.T) {
	dir := generateCachePath()
	storage, err := initFSStorage(dir)
	require.NoError(t, err)
	c, err := InitFSCache(&FSCacheOpts{Path: dir})
	require.NoError(t, err)

	c.Set("hAsH", []byte{1, 2, 3})
	waitForCache()

	c.Remove("hAsH")
	waitForCache()
	_, err = os.Stat(storage.getPath("hAsH"))
	assert.Error(t, err, "file %v unexpectedly found", storage.getPath("hAsH"))
}

func TestFSCacheEviction(t *testing.T) {
	dir := generateCachePath()
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	storage, err := initFSStorage(dir)
	require.NoError(t, err)
	c, err := InitFSCache(&FSCacheOpts{Path: dir, Size: 100})
	require.NoError(t, err)

	c.Set("one", make([]byte, 40))
	waitForCache()

	c.Set("two", make([]byte, 40))
	waitForCache()

	c.Set("three", make([]byte, 40))
	waitForCache()

	assert.False(t, c.Has("one"))

	evictedPath := storage.getPath("one")
	_, err = os.Stat(evictedPath)
	assert.Error(t, err, "file %v unexpectedly found", evictedPath)
}

func TestNewPlayerWithCacheFull(t *testing.T) {
	player := NewPlayer(&Opts{EnableLocalCache: true, EnablePrefetch: false})

	original, err := ioutil.ReadFile("../../downloaded_stream.mp4")
	require.NoError(t, err)

	router := mux.NewRouter()
	router.Path("/content/claims/{uri}/{claim}/{filename}").HandlerFunc(NewRequestHandler(player).Handle)

	uri := "/content/claims/known-size/0590f924bbee6627a2e79f7f2ff7dfb50bf2877c/stream.mp4"
	rng := &rangeHeader{end: 4000000}

	// response := makeRequest(router, http.MethodGet, uri, rng)
	// uncachedData := make([]byte, rng.end+1)
	// read, err := response.Body.Read(uncachedData)
	// assert.Equal(t, http.StatusPartialContent, response.StatusCode)
	// require.NoError(t, err)
	// assert.Equal(t, rng.end+1, read)

	response := makeRequest(router, http.MethodGet, uri, rng)
	cachedData := make([]byte, rng.end+1)
	read, err := response.Body.Read(cachedData)
	assert.NoError(t, err)
	assert.Equal(t, rng.end+1, read)
	assert.Equal(t, cachedData, original[:4000000])

	response = makeRequest(router, http.MethodGet, uri, rng)
	dataFromCache := make([]byte, rng.end+1)
	read, err = response.Body.Read(dataFromCache)
	assert.NoError(t, err)
	assert.Equal(t, rng.end+1, read)
	assert.Equal(t, hex.EncodeToString(dataFromCache), hex.EncodeToString(original[:4000000]))
}
