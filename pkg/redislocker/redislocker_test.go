package redislocker

import (
	"context"
	"log"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OdyseeTeam/odysee-api/pkg/testservices"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tus/tusd/v2/pkg/handler"
)

var redisOpts *redis.Options

func TestMain(m *testing.M) {
	var err error
	var teardown testservices.Teardown
	redisOpts, teardown, err = testservices.Redis()
	if err != nil {
		log.Fatalf("failed to init redis: %s", err)
	}
	defer teardown()

	code := m.Run()
	os.Exit(code)
}

func TestLocker(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	locker, err := New(redisOpts)
	r.NoError(err)

	lock1, err := locker.NewLock("one")
	a.NoError(err)

	a.NoError(lock1.Lock(context.Background(), func() {}))
	a.ErrorIs(lock1.Lock(context.Background(), func() {}), handler.ErrFileLocked)

	lock2, err := locker.NewLock("one")
	a.NoError(err)
	a.ErrorIs(lock2.Lock(context.Background(), func() {}), handler.ErrFileLocked)

	a.NoError(lock1.Unlock())
}

func TestLockerTimeout(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	locker, err := New(redisOpts)
	r.NoError(err)

	lock1, err := locker.NewLock("timeout-test")
	r.NoError(err)
	r.NoError(lock1.Lock(context.Background(), func() {}))

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	lock2, err := locker.NewLock("timeout-test")
	r.NoError(err)
	err = lock2.Lock(ctx, func() {})
	a.ErrorIs(err, handler.ErrLockTimeout)

	a.NoError(lock1.Unlock())
}

func TestLockerRequestRelease(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	locker, err := New(redisOpts)
	r.NoError(err)

	var released atomic.Bool

	lock1, err := locker.NewLock("release-test")
	r.NoError(err)
	r.NoError(lock1.Lock(context.Background(), func() {
		released.Store(true)
	}))

	lock2, err := locker.NewLock("release-test")
	r.NoError(err)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_ = lock2.Lock(ctx, func() {})

	a.True(released.Load(), "requestRelease callback should have been invoked")

	a.NoError(lock1.Unlock())
}
