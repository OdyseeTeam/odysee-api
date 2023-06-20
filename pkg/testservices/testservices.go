package testservices

import (
	"context"
	"fmt"

	"github.com/ory/dockertest/v3"
	"github.com/redis/go-redis/v9"
)

type Teardown func() error

// Redis will spin up a redis container and return a connection options
// plus a tear down function that needs to be called to spin the container down.
func Redis() (*redis.Options, Teardown, error) {
	var err error
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, nil, fmt.Errorf("could not connect to docker: %w", err)
	}

	resource, err := pool.Run("redis", "7", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("could not start resource: %w", err)
	}

	redisOpts := &redis.Options{
		Addr: fmt.Sprintf("localhost:%s", resource.GetPort("6379/tcp")),
	}

	if err = pool.Retry(func() error {
		db := redis.NewClient(redisOpts)
		err := db.Ping(context.Background()).Err()
		return err
	}); err != nil {
		return nil, nil, fmt.Errorf("could not connect to redis: %w", err)
	}

	return redisOpts, func() error {
		if err = pool.Purge(resource); err != nil {
			return fmt.Errorf("could not purge resource: %w", err)
		}
		return nil
	}, nil
}
