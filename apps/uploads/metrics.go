package uploads

import (
	"github.com/prometheus/client_golang/prometheus"
)

const ns = "uploads_v4"

var (
	userAuthErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "user_auth_errors",
	})
	sqlErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "sql_errors",
	})
	redisErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "redis_errors",
	})
)

func registerMetrics() {
	prometheus.MustRegister(
		userAuthErrors, sqlErrors, redisErrors,
	)
}
