package forklift

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const ns = "forklift"
const LabelFatal = "fatal"
const LabelCommon = "common"
const LabelRetrieve = "retrieve"
const LabelAnalyze = "analyze"
const LabelStreamCreate = "stream_create"
const LabelUpstream = "upstream"

var (
	ProcessingTime = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: ns,
		Name:      "processing_time",
		Buckets:   []float64{1, 5, 30, 60, 120, 300, 600},
	}, []string{"stage"})
	ProcessingErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "processing_errors",
	}, []string{"stage"})
)

func RegisterMetrics() {
	prometheus.MustRegister(
		ProcessingTime, ProcessingErrors,
	)
}

func observeDuration(stage string, start time.Time) {
	ProcessingTime.WithLabelValues(stage).Observe(float64(time.Since(start)))
}

func observeError(stage string) {
	ProcessingErrors.WithLabelValues(stage).Inc()
}
