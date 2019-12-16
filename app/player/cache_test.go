package player

import (
	"encoding/hex"
	"net/http"
	"os"
	"path"
	"strings"
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

func TestNewPlayerWithCache(t *testing.T) {
	cachingPlayer := NewPlayer(&Opts{EnableLocalCache: true})

	router := mux.NewRouter()
	playerHandler := NewRequestHandler(cachingPlayer)
	playerRouter := router.Path("/content/claims/{uri}/{claim}/{filename}").Subrouter()
	playerRouter.HandleFunc("", playerHandler.Handle).Methods("GET")

	uri := "/content/claims/what/6769855a9aa43b67086f9ff3c1a5bacb5698a27a/stream.mp4"
	rng := &rangeHeader{4000000, 4000104, 0}
	expected := "6E81C93A90DD3A322190C8D608E29AA929867407596665097B5AE780412" +
		"61638A51C10BC26770AFFEF1533715FBD1428DCADEDC7BEA5D7A9C7D170" +
		"B71EF38E7138D24B0C7E86D791695EDAE1B88EDBE54F95C98EF3DCFD91D" +
		"A025C284EE37D8FEEA2EA84B76B9A22D3"

	response := makeRequest(router, http.MethodGet, uri, rng)
	responseStream := make([]byte, rng.end-rng.start+1)
	require.Equal(t, http.StatusPartialContent, response.StatusCode)
	_, err := response.Body.Read(responseStream)
	require.NoError(t, err)
	assert.Equal(t, strings.ToLower(expected), hex.EncodeToString(responseStream))

	response = makeRequest(router, http.MethodGet, uri, rng)
	responseStream = make([]byte, rng.end-rng.start+1)
	_, err = response.Body.Read(responseStream)
	require.NoError(t, err)
	assert.Equal(t, strings.ToLower(expected), hex.EncodeToString(responseStream))

	response = makeRequest(router, http.MethodGet, uri, rng)
	responseStream = make([]byte, rng.end-rng.start+1)
	_, err = response.Body.Read(responseStream)
	require.NoError(t, err)
	assert.Equal(t, strings.ToLower(expected), hex.EncodeToString(responseStream))

	response = makeRequest(router, http.MethodGet, uri, rng)
	responseStream = make([]byte, rng.end-rng.start+1)
	_, err = response.Body.Read(responseStream)
	require.NoError(t, err)
	assert.Equal(t, strings.ToLower(expected), hex.EncodeToString(responseStream))

	response = makeRequest(router, http.MethodGet, uri, rng)
	responseStream = make([]byte, rng.end-rng.start+1)
	_, err = response.Body.Read(responseStream)
	require.NoError(t, err)
	assert.Equal(t, strings.ToLower(expected), hex.EncodeToString(responseStream))

	cache := cachingPlayer.localCache.(*fsCache)
	assert.EqualValues(t, 4, cache.rCache.Metrics.Hits())
}
