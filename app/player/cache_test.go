package player

import (
	"encoding/hex"
	"net/http"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPlayerWithCache(t *testing.T) {
	cachingPlayer := NewPlayer(&PlayerOpts{EnableLocalCache: true})

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
