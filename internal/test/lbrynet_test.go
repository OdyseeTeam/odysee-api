package test

import (
	"context"
	"testing"

	"github.com/OdyseeTeam/odysee-api/app/query"
	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"

	"github.com/Pallinder/go-randomdata"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

func TestInjectTestingWallet(t *testing.T) {
	userID := randomdata.Number(10000, 90000)
	w, err := InjectTestingWallet(userID)
	require.NoError(t, err)

	c := query.NewCaller(SDKAddress, userID)
	res, err := c.Call(context.Background(), jsonrpc.NewRequest("account_balance"))
	require.NoError(t, err)
	require.Nil(t, res.Error)

	var bal ljsonrpc.AccountBalanceResponse
	err = ljsonrpc.Decode(res.Result, &bal)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, bal.Available.Cmp(decimal.NewFromInt(1)), 0)

	assert.NoError(t, w.Unload())
	assert.NoError(t, w.RemoveFile())
}
