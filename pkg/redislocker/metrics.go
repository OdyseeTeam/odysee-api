package redislocker

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const ns = "redislocker"

var (
	once = sync.Once{}

	locked = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "locked",
	})
	unlocked = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "unlocked",
	})

	fileLockedErrors = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Subsystem: "errors",
		Name:      "file_locked",
	})
	unlockErrors = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Subsystem: "errors",
		Name:      "unlock",
	})
)

func RegisterMetrics() {
	once.Do(func() {
		prometheus.MustRegister(locked, unlocked, fileLockedErrors, unlockErrors)
	})
}
