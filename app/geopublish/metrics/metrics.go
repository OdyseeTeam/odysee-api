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

	EgressBytes = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "egress_bytes_total",
	})
	EgressDuration = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "egress_duration_total",
	})
	AnalysisDuration = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "analysis_duration_total",
	})
	FileSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: ns,
		Name:      "upload_size_mb",
		Help:      "Distribution of upload sizes",
		Buckets:   []float64{1, 100, 1000, 3000, 5000, 10000},
	}, []string{"media_type"})

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

func RegisterMetrics(registry prometheus.Registerer) {
	if registry == nil {
		registry = prometheus.DefaultRegisterer
	}
	registry.MustRegister(
		UploadsCreated, UploadsProcessed, UploadsCanceled, UploadsFailed,
		QueriesSent, QueriesCompleted, QueriesFailed, QueriesErrored,
		QueueTasks, ProcessingTime, ProcessingErrors,
		EgressBytes, EgressDuration, AnalysisDuration, FileSize,
	)
}
