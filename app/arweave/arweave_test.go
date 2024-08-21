package arweave

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

func TestQueryParamsAsMap(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f, err := os.ReadFile("../../claims.json")
	require.NoError(err)
	var resp jsonrpc.RPCResponse
	require.NoError(json.Unmarshal(f, &resp))
	// result := ReplaceAssetUrls("http://odycdn.com", resp.Result, "result.items", "signing_channel.value.thumbnail.url")
	result, err := ReplaceAssetUrls("http://odycdn.com", resp.Result, "items", "value.thumbnail.url")
	require.NoError(err)

	out, err := json.MarshalIndent(result, "", "  ")
	require.NoError(err)
	assert.Regexp(`http://odycdn.com/explore/\w{64}\?filename=\w{64}\.webp`, string(out))
}
