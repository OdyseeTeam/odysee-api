package db

import "sync"
import "sync/atomic"
import "github.com/lbryio/lbrytv/monitor"

type authToken struct {
	sync.Mutex
	queue int32
}

var tokenLocker = make(map[string]*authToken)

var logger = monitor.NewModuleLogger("token_locks")

func lockToken(token string) {
	if _, ok := tokenLocker[token]; !ok {
		tokenLocker[token] = &authToken{}
	}
	tokenLocker[token].Lock()
	atomic.AddInt32(&tokenLocker[token].queue, 1)
	logger.LogF(monitor.F{monitor.TokenF: token}).Debug("token locked, queue: ", tokenLocker[token].queue)
}

func releaseToken(token string) {
	if _, ok := tokenLocker[token]; ok {
		atomic.AddInt32(&tokenLocker[token].queue, -1)
		tokenLocker[token].Unlock()
		logger.LogF(monitor.F{monitor.TokenF: token}).Debug("token unlocked, queue: ", tokenLocker[token].queue)
		if tokenLocker[token].queue <= 0 {
			delete(tokenLocker, token)
		}
	}
}
