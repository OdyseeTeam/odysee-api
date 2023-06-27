package redislocker

import (
	"github.com/prometheus/client_golang/prometheus"
)

const ns = "redislocker"

var (
	locked = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "locked",
	})
	unlocked = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "unlocked",
	})

	fileLockedErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Subsystem: "errors",
		Name:      "file_locked",
	})
	unlockErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Subsystem: "errors",
		Name:      "unlock",
	})
)

func RegisterMetrics(registry prometheus.Registerer) {
	if registry == nil {
		registry = prometheus.DefaultRegisterer
	}
	registry.MustRegister(locked, unlocked, fileLockedErrors, unlockErrors)
}
