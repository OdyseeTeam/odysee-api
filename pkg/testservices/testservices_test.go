package testservices

import (
	"testing"

	"github.com/OdyseeTeam/odysee-api/app/query"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

func TestLbrynet(t *testing.T) {
	addr, teardown, err := Lbrynet()
	require.NoError(t, err)
	defer teardown()
	c := query.NewCaller(addr, 0)
	res, err := c.Call(jsonrpc.NewRequest("resolve", map[string]interface{}{"urls": "what"}))
	require.NoError(t, err)
	require.Nil(t, res.Error)
}
