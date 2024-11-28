package query

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

func TestGetCacheKey(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	seen := map[string]bool{}
	params := []map[string]any{{}, {"uri": "what"}, {"uri": "odysee"}, nil}
	genCacheKey := func(params map[string]any) string {
		req := jsonrpc.NewRequest(MethodResolve, params)
		query, err := NewQuery(req, "")
		require.NoError(err)
		cacheReq := CacheRequest{
			Method: query.Method(),
			Params: query.Params(),
		}
		return cacheReq.GetCacheKey()
	}
	for _, p := range params {
		t.Run(fmt.Sprintf("%+v", p), func(t *testing.T) {
			cacheKey := genCacheKey(p)
			assert.Len(cacheKey, 32)
			assert.NotContains(seen, cacheKey)
			seen[cacheKey] = true
		})
	}
	assert.Contains(seen, genCacheKey(params[1]))
}
