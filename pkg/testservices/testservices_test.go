package testservices

import (
	"context"
	"testing"

	"github.com/OdyseeTeam/odysee-api/app/query"

	goredislib "github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

func TestRedis(t *testing.T) {
	redisOpts, teardown, err := Redis()
	require.NoError(t, err)
	defer teardown()
	client := goredislib.NewClient(redisOpts)
	err = client.Ping(context.Background()).Err()
	require.NoError(t, err)
}

func TestLbrynet(t *testing.T) {
	addr, teardown, err := Lbrynet()
	require.NoError(t, err)
	defer teardown()
	c := query.NewCaller(addr, 0)
	res, err := c.Call(context.Background(), jsonrpc.NewRequest("resolve", map[string]interface{}{"urls": "what"}))
	require.NoError(t, err)
	require.Nil(t, res.Error)
}
