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
	DraftsCreated = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "drafts_created_total",
	})
	CommitsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "commits_total",
	})
	CommitEnqueueFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "commit_enqueue_failed_total",
	})
)

func registerMetrics() {
	prometheus.MustRegister(
		InternalErrors, QueriesSent, QueriesCompleted, QueriesFailed, QueriesErrored,
		DraftsCreated, CommitsTotal, CommitEnqueueFailed,
	)
}
