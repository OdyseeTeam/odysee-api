package queue

import (
	"github.com/prometheus/client_golang/prometheus"
)

const ns = "queue"

var (
	queueTasks = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: ns,
		Name:      "queue_tasks",
	}, []string{"status"})
)

func registerMetrics(registry prometheus.Registerer) {
	if registry == nil {
		registry = prometheus.DefaultRegisterer
	}
	registry.MustRegister(
		queueTasks,
	)
}
