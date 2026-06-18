package testservices

import (
	"fmt"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type Teardown func() error

// Redis will start a Redis-compatible test server and return connection options
// plus a tear down function that needs to be called to stop the server.
func Redis() (*redis.Options, Teardown, error) {
	var err error
	server, err := miniredis.Run()
	if err != nil {
		return nil, nil, fmt.Errorf("could not start redis: %w", err)
	}

	redisOpts := &redis.Options{
		Addr: server.Addr(),
	}

	return redisOpts, func() error {
		server.Close()
		return nil
	}, nil
}
