package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const ns = "geopublish"
const LabelFatal = "fatal"
const LabelCommon = "common"
const LabelProcessingTotal = "total"
const LabelProcessingAnalyze = "analyze"
const LabelProcessingBlobSplit = "blob_split"
const LabelProcessingReflection = "reflection"
const LabelProcessingQuery = "query"

const LabelType = "type"

var (
	UploadsCreated = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "uploads_created",
	})
	UploadsProcessed = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "uploads_processed",
	})
	UploadsCanceled = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "uploads_canceled",
	})
	UploadsFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "uploads_failed",
	})

	Errors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "errors",
	}, []string{"type"})

	UploadsDBErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "uploads_db_errors",
	})
	QueriesSent = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "queries_sent",
	})
	QueriesCompleted = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "queries_completed",
	})
	QueriesFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "queries_failed",
	})
	QueriesErrored = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "queries_errored",
	})

	QueueTasks = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: ns,
		Name:      "queue_tasks",
	}, []string{"status"})

	ProcessingTime = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: ns,
		Name:      "processing_time",
		Buckets:   []float64{1, 5, 15, 30, 45, 60, 120, 240, 300, 600, 1200},
	}, []string{"stage"})
	ProcessingSpeed = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: ns,
		Name:      "processing_speed",
		Buckets:   []float64{1, 2, 3, 4, 5},
	})
	ProcessingErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "processing_errors",
	}, []string{"stage"})
)

func RegisterMetrics() {
	prometheus.MustRegister(
		UploadsCreated, UploadsProcessed, UploadsCanceled, UploadsFailed,
		QueriesSent, QueriesCompleted, QueriesFailed, QueriesErrored,
		QueueTasks, ProcessingTime, ProcessingSpeed, ProcessingErrors,
	)
}
