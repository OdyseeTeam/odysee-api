package redislocker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	goredislib "github.com/redis/go-redis/v9"
	"github.com/tus/tusd/v2/pkg/handler"
)

var (
	lockTimeout = 100 * time.Second
)

// Locker implements tusd's handler.Locker using Redis via redsync.
// The requestRelease callback (tusd v2) is tracked in-process only.
// Cross-process contention falls back to retry/timeout via redsync.
type Locker struct {
	rs       *redsync.Redsync
	holders  map[string]func()
	holderMu sync.Mutex
}

type lock struct {
	locker *Locker
	name   string
	mutex  *redsync.Mutex
}

func New(redisOpts *goredislib.Options) (*Locker, error) {
	client := goredislib.NewClient(redisOpts)
	err := client.Ping(context.Background()).Err()
	if err != nil {
		return nil, err
	}
	pool := goredis.NewPool(client)
	rs := redsync.New(pool)
	return &Locker{rs: rs, holders: make(map[string]func())}, nil
}

func (locker *Locker) NewLock(name string) (handler.Lock, error) {
	m := locker.rs.NewMutex(name, redsync.WithExpiry(lockTimeout))
	return &lock{locker: locker, name: name, mutex: m}, nil
}

// UseIn adds this locker to the passed composer.
func (locker *Locker) UseIn(composer *handler.StoreComposer) {
	composer.UseLocker(locker)
}

func (l *lock) Lock(ctx context.Context, requestRelease func()) error {
	l.locker.holderMu.Lock()
	if existing, ok := l.locker.holders[l.name]; ok {
		existing()
	}
	l.locker.holderMu.Unlock()

	if err := l.mutex.LockContext(ctx); err != nil {
		fileLockedErrors.Inc()
		if ctx.Err() != nil {
			return fmt.Errorf("%w: file %s: %s", handler.ErrLockTimeout, l.name, err)
		}
		return fmt.Errorf("%w: file %s: %s", handler.ErrFileLocked, l.name, err)
	}

	l.locker.holderMu.Lock()
	l.locker.holders[l.name] = requestRelease
	l.locker.holderMu.Unlock()

	locked.Inc()
	return nil
}

func (l *lock) Unlock() error {
	l.locker.holderMu.Lock()
	delete(l.locker.holders, l.name)
	l.locker.holderMu.Unlock()

	if ok, err := l.mutex.Unlock(); !ok || err != nil {
		unlockErrors.Inc()
		return fmt.Errorf("cannot unlock file %s: %w", l.name, err)
	}
	unlocked.Inc()
	return nil
}
