package asynquery

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	ns          = "asynquery"
	labelAreaDB = "db"
)
const LabelFatal = "fatal"
const LabelCommon = "common"
const LabelProcessingTotal = "total"
const LabelProcessingAnalyze = "analyze"
const LabelProcessingBlobSplit = "blob_split"
const LabelProcessingReflection = "reflection"
const LabelProcessingQuery = "query"

var (
	InternalErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "errors",
	}, []string{"area"})
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

	// QueueLength = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	// 	Namespace: ns,
	// 	Name:      "queue_length",
	// }, []string{"status"})

	// ProcessingTime = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	// 	Namespace: ns,
	// 	Name:      "processing_time",
	// 	Buckets:   []float64{1, 5, 30, 60, 120, 300, 600},
	// }, []string{"stage"})
	// ProcessingErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
	// 	Namespace: ns,
	// 	Name:      "processing_errors",
	// }, []string{"stage"})
)

func registerServerMetrics() {
	prometheus.MustRegister(
		QueriesSent, QueriesCompleted, QueriesFailed, QueriesErrored,
	)
}
