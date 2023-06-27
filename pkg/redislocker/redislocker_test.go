package redislocker

import (
	"log"
	"os"
	"testing"

	"github.com/OdyseeTeam/odysee-api/pkg/testservices"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tus/tusd/pkg/handler"
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

	a.NoError(lock1.Lock())
	a.ErrorIs(lock1.Lock(), handler.ErrFileLocked)

	lock2, err := locker.NewLock("one")
	a.NoError(err)
	a.ErrorIs(lock2.Lock(), handler.ErrFileLocked)

	a.NoError(lock1.Unlock())
}
