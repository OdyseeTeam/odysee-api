package query

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	CacheOperationGet = "get"
	CacheOperationSet = "set"

	CacheResultHit     = "hit"
	CacheResultMiss    = "miss"
	CacheResultSuccess = "success"
	CacheResultError   = "error"
)

var (
	queryRetrievalDurationBuckets = []float64{0.025, 0.05, 0.1, 0.25, 0.4, 1, 2.5, 5, 10, 25, 50, 100, 300}
	cacheDurationBuckets          = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0}

	QueryCacheOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "query_cache",
			Name:      "operation_duration_seconds",
			Help:      "Cache operation latency",
			Buckets:   cacheDurationBuckets,
		},
		[]string{"operation", "result", "method"},
	)
	QueryRetrievalDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "query_cache",
			Name:      "retrieval_duration_seconds",
			Help:      "Latency for cold cache retrieval",
			Buckets:   queryRetrievalDurationBuckets,
		},
		[]string{"result", "method"},
	)
)

func ObserveQueryCacheOperation(operation, result, method string, start time.Time) {
	QueryCacheOperationDuration.WithLabelValues(operation, result, method).Observe(float64(time.Since(start).Seconds()))
}

func ObserveQueryRetrievalDuration(result, method string, start time.Time) {
	QueryRetrievalDuration.WithLabelValues(result, method).Observe(float64(time.Since(start).Seconds()))
}
