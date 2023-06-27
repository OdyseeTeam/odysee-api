package testdeps

import (
	"context"
	"strconv"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

const (
	baseRedisTestURL = "redis://:odyredis@localhost:6379/"
)

type RedisTestHelper struct {
	AsynqOpts asynq.RedisConnOpt
	Client    *redis.Client
	Opts      *redis.Options
	URL       string
}

func NewRedisTestHelper(t *testing.T, args ...int) *RedisTestHelper {
	t.Helper()
	var db int

	if len(args) > 0 {
		db = args[0]
	}
	url := baseRedisTestURL + strconv.Itoa(db)
	redisOpts, err := redis.ParseURL(url)

	require.NoError(t, err)
	asynqOpts, err := asynq.ParseRedisURI(url)
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
		URL:       url,
	}
}
