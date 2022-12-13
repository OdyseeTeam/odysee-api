package forklift

import (
	"github.com/prometheus/client_golang/prometheus"
)

const ns = "forklift"
const LabelFatal = "fatal"
const LabelCommon = "common"
const LabelProcessingTotal = "total"
const LabelProcessingAnalyze = "analyze"
const LabelProcessingBlobSplit = "blob_split"
const LabelProcessingReflection = "reflection"
const LabelProcessingQuery = "query"

var (
	QueueTasks = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: ns,
		Name:      "queue_tasks",
	}, []string{"status"})

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
		QueueTasks, ProcessingTime, ProcessingErrors,
	)
}
