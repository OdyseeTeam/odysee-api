package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const ns = "geopublish"
const LabelFatal = "fatal"
const LabelCommon = "common"

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
	BlobUploadErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "blob_upload_errors",
	}, []string{"type"})

	QueueTasks = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: ns,
		Name:      "queue_tasks",
	}, []string{"status"})
)

func RegisterMetrics() {
	prometheus.MustRegister(
		UploadsCreated, UploadsProcessed, UploadsCanceled, UploadsFailed,
		QueriesSent, QueriesCompleted, QueriesFailed, QueriesErrored,
		BlobUploadErrors, QueueTasks,
	)
}
