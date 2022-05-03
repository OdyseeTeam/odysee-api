package watchman

import (
	"github.com/prometheus/client_golang/prometheus"
)

var httpResponses = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "watchman_http_responses",
		Help:    "Method call latency distributions",
		Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.4, 1, 2, 5, 10},
	},
	[]string{"endpoint", "status_code"},
)

func RegisterMetrics() {
	prometheus.MustRegister(httpResponses)
}
