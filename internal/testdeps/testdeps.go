package testdeps

import (
	"context"
	"testing"

	"github.com/go-redis/redis/v8"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"
)

const (
	RedisTestURL = "redis://:odyredis@localhost:6379/0"
)

type RedisTestHelper struct {
	AsynqOpts asynq.RedisConnOpt
	Client    *redis.Client
	Opts      *redis.Options
	URL       string
}

func NewRedisTestHelper(t *testing.T) *RedisTestHelper {
	t.Helper()
	redisOpts, err := redis.ParseURL(RedisTestURL)
	require.NoError(t, err)
	asynqOpts, err := asynq.ParseRedisURI(RedisTestURL)
	require.NoError(t, err)
	c := redis.NewClient(redisOpts)
	c.FlushDB(context.Background())
	t.Cleanup(func() {
		c.FlushDB(context.Background())
		c.Close()
	})
	return &RedisTestHelper{
		AsynqOpts: asynqOpts,
		Client:    c,
		Opts:      redisOpts,
		URL:       RedisTestURL,
	}
}
