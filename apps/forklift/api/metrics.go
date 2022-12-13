package api

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	ns = "forklift_api"

	areaAuth       = "auth"
	areaInput      = "input"
	areaObjectLock = "object_lock"
	areaObjectGet  = "object_get"
	areaObjectMeta = "object_meta"
	areaStorage    = "storage"
	areaQueue      = "queue"
	areaDB         = "db"
)

var (
	metricErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "processing_errors",
	}, []string{"area"})
)

func RegisterMetrics() {
	prometheus.MustRegister(
		metricErrors,
	)
}
