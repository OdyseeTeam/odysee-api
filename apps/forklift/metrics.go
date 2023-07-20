package forklift

import (
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const ns = "forklift"
const LabelFatal = "fatal"
const LabelCommon = "common"
const LabelRetrieve = "retrieve"
const LabelAnalyze = "analyze"
const LabelStreamCreate = "stream_create"
const LabelUpstream = "upstream"

var onceMetrics sync.Once

var (
	waitTimeMinutes = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: ns,
		Name:      "wait_time_minutes",
		Buckets:   []float64{1, 5, 10, 15, 20, 30, 45, 60, 120},
	})
	processingDurationSeconds = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "processing_duration_seconds",
	}, []string{"stage"})
	processingErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "processing_errors",
	}, []string{"stage"})

	egressVolumeMB = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "egress_volume_mb",
	})
	egressDurationSeconds = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "egress_duration_seconds",
	})
)

func registerMetrics(registry prometheus.Registerer) {
	if registry == nil {
		registry = prometheus.DefaultRegisterer
	}
	registry.MustRegister(
		waitTimeMinutes, processingDurationSeconds, processingErrors, egressVolumeMB, egressDurationSeconds,
	)
}

func BuildMetricsHandler() http.Handler {
	registry := prometheus.NewRegistry()
	onceMetrics.Do(func() {
		registerMetrics(registry)
	})
	return promhttp.InstrumentMetricHandler(
		registry, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
	)
}

func observeDuration(stage string, start time.Time) {
	processingDurationSeconds.WithLabelValues(stage).Add(float64(time.Since(start)))
}

func observeError(stage string) {
	processingErrors.WithLabelValues(stage).Inc()
}
