package iapi

import (
	"fmt"
	"testing"

	"github.com/OdyseeTeam/odysee-api/app/wallet"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCall(t *testing.T) {
	claimID := "81b1749f773bad5b9b53d21508051560f2746cdc"
	oat, err := wallet.GetTestToken()
	require.NoError(t, err)
	c, err := NewClient(WithOAuthToken(oat.AccessToken))
	require.NoError(t, err)
	r := &CustomerListResponse{}
	err = c.Call(
		"customer/list",
		map[string]string{"target_claim_id_filter": claimID},
		r)
	require.NoError(t, err)
	assert.True(t, r.Success)
	assert.Nil(t, r.Error)
	fmt.Println(r.Data)
	assert.Equal(t, claimID, r.Data[0].TargetClaimID)
}
