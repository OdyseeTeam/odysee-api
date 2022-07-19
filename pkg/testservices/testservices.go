package testservices

import (
	"context"
	"fmt"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/query"
	"github.com/go-redis/redis/v8"
	"github.com/ory/dockertest/v3"
	"github.com/ybbus/jsonrpc"
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

// Redis will spin up a redis container and return a connection options
// plus a tear down function that needs to be called to spin the container down.
func Lbrynet() (string, Teardown, error) {
	var err error
	pool, err := dockertest.NewPool("")
	pool.MaxWait = 120 * time.Second
	if err != nil {
		return "", nil, fmt.Errorf("could not connect to docker: %w", err)
	}

	resource, err := pool.Run("odyseeteam/lbrynet-tv", "0.101.2", []string{"SDK_CONFIG=/daemon/daemon_settings.yml"})
	if err != nil {
		return "", nil, fmt.Errorf("could not start resource: %w", err)
	}

	addr := fmt.Sprintf("http://localhost:%s", resource.GetPort("5279/tcp"))
	teardown := func() error {
		if err = pool.Purge(resource); err != nil {
			return fmt.Errorf("could not purge resource: %w", err)
		}
		return nil
	}

	if err = pool.Retry(func() error {
		c := query.NewCaller(addr, 0)
		rpcRes, err := c.Call(jsonrpc.NewRequest("resolve", map[string]interface{}{"urls": "what"}))

		if err != nil {
			return err
		} else if rpcRes.Error != nil {
			return fmt.Errorf("%s", rpcRes.Error.Message)
		}
		return nil
	}); err != nil {
		teardown()
		return "", nil, fmt.Errorf("could not connect to lbrynet: %w", err)
	}

	return addr, teardown, nil
}
