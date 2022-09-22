package testservices

import (
	"context"
	"testing"

	goredislib "github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func TestRedis(t *testing.T) {
	redisOpts, teardown, err := Redis()
	require.NoError(t, err)
	defer teardown()
	client := goredislib.NewClient(redisOpts)
	err = client.Ping(context.Background()).Err()
	require.NoError(t, err)
}
