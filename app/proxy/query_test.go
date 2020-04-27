package proxy

import (
	"testing"

	"github.com/lbryio/lbrytv/internal/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

func TestQueryParamsAsMap(t *testing.T) {
	q, err := NewQuery(jsonrpc.NewRequest("version"))
	require.NoError(t, err)
	assert.Nil(t, q.ParamsAsMap())

	q, err = NewQuery(jsonrpc.NewRequest("resolve", map[string]interface{}{"urls": "what"}))
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"urls": "what"}, q.ParamsAsMap())

	q, err = NewQuery(jsonrpc.NewRequest("account_balance"))
	require.NoError(t, err)

	q.WalletID = "123"
	err = q.validate()
	require.NoError(t, err, errors.Unwrap(err))
	assert.Equal(t, map[string]interface{}{"wallet_id": "123"}, q.ParamsAsMap())

	searchParams := map[string]interface{}{
		"any_tags": []interface{}{
			"art", "automotive", "blockchain", "comedy", "economics", "education",
			"gaming", "music", "news", "science", "sports", "technology",
		},
	}
	q, err = NewQuery(jsonrpc.NewRequest("claim_search", searchParams))
	require.NoError(t, err)
	assert.Equal(t, searchParams, q.ParamsAsMap())
}
