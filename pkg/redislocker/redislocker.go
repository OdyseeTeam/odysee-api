package redislocker

import (
	"context"
	"fmt"
	"time"

	goredislib "github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v8"
	"github.com/tus/tusd/pkg/handler"
)

var (
	lockTimeout = 100 * time.Second
)

type Locker struct {
	rs *redsync.Redsync
}

type lock struct {
	name  string
	mutex *redsync.Mutex
}

func New(redisOpts *goredislib.Options) (*Locker, error) {
	client := goredislib.NewClient(redisOpts)
	err := client.Ping(context.Background()).Err()
	if err != nil {
		return nil, err
	}
	pool := goredis.NewPool(client)
	rs := redsync.New(pool)
	return &Locker{rs}, nil
}

func (locker *Locker) NewLock(name string) (handler.Lock, error) {
	m := locker.rs.NewMutex(name, redsync.WithExpiry(lockTimeout))
	return &lock{name, m}, nil
}

// UseIn adds this locker to the passed composer.
func (locker *Locker) UseIn(composer *handler.StoreComposer) {
	composer.UseLocker(locker)
}

func (l lock) Lock() error {
	if err := l.mutex.Lock(); err != nil {
		fileLockedErrors.Inc()
		return fmt.Errorf("%w: file %s: %s", handler.ErrFileLocked, l.name, err)
	}
	locked.Inc()
	return nil
}

func (l lock) Unlock() error {
	if ok, err := l.mutex.Unlock(); !ok || err != nil {
		unlockErrors.Inc()
		return fmt.Errorf("cannot unlock file %s: %w", l.name, err)
	}
	unlocked.Inc()
	return nil
}
