package proxy

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryParamsAsMap(t *testing.T) {
	q, err := NewQuery(newRawRequest(t, "version", nil))
	require.NoError(t, err)
	assert.Nil(t, q.ParamsAsMap())

	q, err = NewQuery(newRawRequest(t, "resolve", map[string]string{"urls": "what"}))
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"urls": "what"}, q.ParamsAsMap())

	q, err = NewQuery(newRawRequest(t, "account_balance", nil))
	require.NoError(t, err)

	q.SetWalletID("123")
	err = q.validate()
	require.NoError(t, err, errors.Unwrap(err))
	assert.Equal(t, map[string]interface{}{"wallet_id": "123"}, q.ParamsAsMap())

	searchParams := map[string]interface{}{
		"any_tags": []interface{}{
			"art", "automotive", "blockchain", "comedy", "economics", "education",
			"gaming", "music", "news", "science", "sports", "technology",
		},
	}
	q, err = NewQuery(newRawRequest(t, "claim_search", searchParams))
	require.NoError(t, err)
	assert.Equal(t, searchParams, q.ParamsAsMap())
}
