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

	CacheAreaChainquery     = "chainquery"
	CacheAreaInvalidateCall = "invalidate_call"

	CacheRetrieverErrorNet   = "net"
	CacheRetrieverErrorSdk   = "sdk"
	CacheRetrieverErrorInput = "input"
)

var (
	QueryCacheRetrievalDurationBuckets = []float64{0.025, 0.05, 0.1, 0.25, 0.4, 1, 2.5, 5, 10, 25, 50, 100, 300}
	cacheDurationBuckets               = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0}

	QueryCacheOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "query_cache",
			Name:      "operation_duration_seconds",
			Help:      "Cache operation latency",
			Buckets:   cacheDurationBuckets,
		},
		[]string{"operation", "result", "method"},
	)
	QueryCacheRetrievalDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "query_cache",
			Name:      "retrieval_duration_seconds",
			Help:      "Latency for cold cache retrieval",
			Buckets:   QueryCacheRetrievalDurationBuckets,
		},
		[]string{"result", "method"},
	)
	QueryCacheRetrievalFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "query_cache",
			Name:      "retrieval_retries",
			Help:      "Retries for cold cache retrieval",
		},
		[]string{"kind", "method"},
	)
	QueryCacheRetrySuccesses = promauto.NewSummary(
		prometheus.SummaryOpts{
			Namespace: "query_cache",
			Name:      "retry_successes",
			Help:      "Successful counts of cache retrieval retries",
		},
	)
	QueryCacheErrorCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "query_cache",
			Name:      "error_count",
			Help:      "Errors unrelated to cache setting/retrieval",
		},
		[]string{"area"},
	)
)

func ObserveQueryCacheOperation(operation, result, method string, start time.Time) {
	QueryCacheOperationDuration.WithLabelValues(operation, result, method).Observe(float64(time.Since(start).Seconds()))
}

func ObserveQueryCacheRetrievalDuration(result, method string, start time.Time) {
	QueryCacheRetrievalDuration.WithLabelValues(result, method).Observe(float64(time.Since(start).Seconds()))
}
