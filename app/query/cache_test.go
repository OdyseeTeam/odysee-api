package query

import (
	"context"
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc/v2"
)

// deadlineCache models a cache backend that rejects expired operations.
type deadlineCache struct {
	cache.CacheInterface[any]
	entries map[string]any
}

func (c *deadlineCache) Get(ctx context.Context, key any) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	value, ok := c.entries[key.(cache.CacheKeyGenerator).GetCacheKey()]
	if !ok {
		return nil, &store.NotFound{}
	}
	return value, nil
}

func (c *deadlineCache) Set(ctx context.Context, key, value any, _ ...store.Option) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.entries[key.(cache.CacheKeyGenerator).GetCacheKey()] = value
	return nil
}

func TestRetrieveCachesSlowGetter(t *testing.T) {
	for _, delay := range []time.Duration{0, 6 * time.Second} {
		t.Run(delay.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				backend := &deadlineCache{entries: make(map[string]any)}
				queryCache := NewQueryCache(backend)
				query, err := NewQuery(jsonrpc.NewRequest(MethodResolve, map[string]any{ParamUrls: []string{"what"}}), "")
				require.NoError(t, err)
				result := map[string]any{"resolved": "lbry://what"}
				calls := 0
				getter := func() (any, error) {
					calls++
					// Advance fake time beyond the cache lookup's five-second deadline.
					time.Sleep(delay)
					return &jsonrpc.RPCResponse{Result: result}, nil
				}

				first, err := queryCache.Retrieve(query, getter)
				require.NoError(t, err)
				require.NotNil(t, first)
				assert.Equal(t, result, first.Result)
				second, err := queryCache.Retrieve(query, getter)
				require.NoError(t, err)
				require.NotNil(t, second)
				assert.Equal(t, first, second)
				assert.Equal(t, 1, calls, "the successful response should be reused from cache")
			})
		})
	}
}

func TestGetCacheKey(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	seen := map[string]bool{}
	params := []map[string]any{
		{},
		{"uri": "what"},
		{"uri": "odysee"},
		nil,
	}
	genCacheKey := func(params map[string]any, metaKey string) string {
		req := jsonrpc.NewRequest(MethodResolve, params)
		query, err := NewQuery(req, "")
		require.NoError(err)
		cacheReq := NewCacheRequest(query.Method(), query.Params(), metaKey)
		return cacheReq.GetCacheKey()
	}
	for _, p := range params {
		t.Run(fmt.Sprintf("%+v", p), func(t *testing.T) {
			cacheKey := genCacheKey(p, "")
			assert.Len(cacheKey, 32)
			assert.NotContains(seen, cacheKey)
			seen[cacheKey] = true
		})
		t.Run(fmt.Sprintf("%+v", p), func(t *testing.T) {
			cacheKey := genCacheKey(p, "user@endpoint")
			assert.Len(cacheKey, 32)
			assert.NotContains(seen, cacheKey)
			seen[cacheKey] = true
		})
	}
	assert.Contains(seen, genCacheKey(params[1], "user@endpoint"))
}

func TestCachedResponseMarshal(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	jr, err := decodeResponse(resolveResponseWithPurchase)
	require.NoError(err)
	require.NotNil(jr.Result)
	r := &CachedResponse{
		Result: jr.Result,
		Error:  jr.Error,
	}
	mr, err := r.MarshalBinary()
	require.NoError(err)
	require.NotEmpty(mr)
	assert.Less(len(mr), len(resolveResponseWithPurchase))
	r2 := &CachedResponse{}
	err = r2.UnmarshalBinary(mr)
	require.NoError(err)
	assert.Equal(r, r2)
}
