package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	dto "github.com/prometheus/client_model/go"
)

const (
	nsPlayer     = "player"
	nsAPI        = "api"
	nsIAPI       = "iapi"
	nsAuth       = "auth"
	nsProxy      = "proxy"
	nsLbrynext   = "lbrynext"
	nsLbrynet    = "lbrynet"
	nsUI         = "ui"
	nsLbrytv     = "lbrytv"
	nsOperations = "op"

	LabelSource   = "source"
	LabelInstance = "instance"

	LabelNameType  = "type"
	LabelValuePaid = "paid"
	LabelValueFree = "free"

	FailureKindNet = "net"
	FailureKindRPC = "rpc"
	// FailureKindRPCJSON is not called FailureKindJSONRPC because this is an error from RPC server, just pertinent to JSON serialization.
	FailureKindRPCJSON          = "rpc_json"
	FailureKindClientJSON       = "client_json"
	FailureKindClient           = "client"
	FailureKindAuth             = "auth"
	FailureKindInternal         = "internal"
	FailureKindLbrynetXMismatch = "xmismatch"

	GroupControl      = "control"
	GroupExperimental = "experimental"
)

var (
	callsSecondsBuckets = []float64{0.005, 0.025, 0.05, 0.1, 0.25, 0.4, 1, 2, 5, 10, 20, 60, 120, 300}

	IAPIAuthSuccessDurations = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: nsIAPI,
		Subsystem: "auth",
		Name:      "success_seconds",
		Help:      "Time to successful authentication",
	})
	IAPIAuthFailedDurations = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: nsIAPI,
		Subsystem: "auth",
		Name:      "failed_seconds",
		Help:      "Time to failed authentication response",
	})
	IAPIAuthErrorDurations = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: nsIAPI,
		Subsystem: "auth",
		Name:      "error_seconds",
		Help:      "Time to failed authentication response",
	})

	AuthTokenCacheHits = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: nsAuth,
		Subsystem: "cache",
		Name:      "hits",
	})
	AuthTokenCacheMisses = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: nsAuth,
		Subsystem: "cache",
		Name:      "misses",
	})

	ProxyE2ECallDurations = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: nsProxy,
			Subsystem: "e2e_calls",
			Name:      "total_seconds",
			Help:      "End-to-end method call latency distributions",
			Buckets:   callsSecondsBuckets,
		},
		[]string{"method"},
	)
	ProxyE2ECallFailedDurations = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: nsProxy,
			Subsystem: "e2e_calls",
			Name:      "failed_seconds",
			Help:      "Failed end-to-end method call latency distributions",
			Buckets:   callsSecondsBuckets,
		},
		[]string{"method", "kind"},
	)
	ProxyE2ECallCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: nsProxy,
			Subsystem: "e2e_calls",
			Name:      "total_count",
			Help:      "End-to-end method call count",
		},
		[]string{"method"},
	)
	ProxyE2ECallFailedCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: nsProxy,
			Subsystem: "e2e_calls",
			Name:      "failed_count",
			Help:      "Failed end-to-end method call count",
		},
		[]string{"method", "kind"},
	)

	ProxyCallDurations = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: nsProxy,
			Subsystem: "calls",
			Name:      "total_seconds",
			Help:      "Method call latency distributions",
			Buckets:   callsSecondsBuckets,
		},
		[]string{"method", "endpoint", "origin"},
	)
	ProxyCallFailedDurations = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: nsProxy,
			Subsystem: "calls",
			Name:      "failed_seconds",
			Help:      "Failed method call latency distributions",
			Buckets:   callsSecondsBuckets,
		},
		[]string{"method", "endpoint", "origin", "kind"},
	)
	ProxyCallCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: nsProxy,
			Subsystem: "calls",
			Name:      "total_count",
			Help:      "Method call count",
		},
		[]string{"method", "endpoint", "origin"},
	)
	ProxyCallFailedCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: nsProxy,
			Subsystem: "calls",
			Name:      "failed_count",
			Help:      "Failed method call count",
		},
		[]string{"method", "endpoint", "origin", "kind"},
	)

	ProxyQueryCacheHitCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: nsProxy,
		Subsystem: "cache",
		Name:      "hit_count",
		Help:      "Total number of queries found in the local cache",
	}, []string{"method"})
	ProxyQueryCacheMissCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: nsProxy,
		Subsystem: "cache",
		Name:      "miss_count",
		Help:      "Total number of queries that were not in the local cache",
	}, []string{"method"})
	ProxyQueryCacheErrorCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: nsProxy,
		Subsystem: "cache",
		Name:      "error_count",
		Help:      "Total number of errors retrieving queries from the local cache",
	}, []string{"method"})

	LbrynetWalletsLoaded = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: nsLbrynet,
		Subsystem: "wallets",
		Name:      "count",
		Help:      "Number of wallets currently loaded",
	}, []string{LabelSource})

	UIBufferCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: nsUI,
		Subsystem: "content",
		Name:      "buffer_count",
		Help:      "Video buffer events",
	})
	UITimeToStart = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: nsUI,
		Subsystem: "content",
		Name:      "time_to_start",
		Help:      "How long it takes the video to start",
		Buckets:   []float64{0.1, 0.25, 0.5, 1, 2, 4, 8, 16, 32},
	})

	LbrytvCallDurations = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: nsLbrytv,
			Subsystem: "calls",
			Name:      "total_seconds",
			Help:      "How long do calls to lbrytv take (end-to-end)",
			Buckets:   callsSecondsBuckets,
		},
		[]string{"path"},
	)

	LbrytvNewUsers = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: nsLbrytv,
		Subsystem: "users",
		Name:      "count",
		Help:      "Total number of new users created in the database",
	})
	LbrytvPurchases = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: nsLbrytv,
		Subsystem: "purchase",
		Name:      "count",
		Help:      "Total number of purchases done",
	})
	LbrytvPurchaseAmounts = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: nsLbrytv,
		Subsystem: "purchase",
		Name:      "amounts",
		Help:      "Purchase amounts",
		Buckets:   []float64{1, 10, 100, 1000, 10000},
	})
	LbrytvStreamRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: nsLbrytv,
		Subsystem: "stream",
		Name:      "count",
		Help:      "Total number of stream requests received",
	}, []string{LabelNameType})

	LbrytvDBOpenConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: nsLbrytv,
		Subsystem: "db",
		Name:      "conns_open",
		Help:      "Number of open db connections in the Go connection pool",
	})
	LbrytvDBInUseConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: nsLbrytv,
		Subsystem: "db",
		Name:      "conns_in_use",
		Help:      "Number of in-use db connections in the Go connection pool",
	})
	LbrytvDBIdleConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: nsLbrytv,
		Subsystem: "db",
		Name:      "conns_idle",
		Help:      "Number of idle db connections in the Go connection pool",
	})

	LbrynetXCallDurations = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: nsLbrynext,
			Subsystem: "calls",
			Name:      "total_seconds",
			Help:      "Method call latency distributions",
			Buckets:   callsSecondsBuckets,
		},
		[]string{"method", "endpoint", "group"},
	)
	LbrynetXCallFailedDurations = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: nsLbrynext,
			Subsystem: "calls",
			Name:      "failed_seconds",
			Help:      "Failed method call latency distributions",
			Buckets:   callsSecondsBuckets,
		},
		[]string{"method", "endpoint", "group", "kind"},
	)
	LbrynetXCallCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: nsLbrynext,
			Subsystem: "calls",
			Name:      "total_count",
			Help:      "Method call count",
		},
		[]string{"method", "endpoint", "group"},
	)
	LbrynetXCallFailedCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: nsLbrynext,
			Subsystem: "calls",
			Name:      "failed_count",
			Help:      "Failed method call count",
		},
		[]string{"method", "endpoint", "group", "kind"},
	)

	operations = promauto.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: nsOperations,
			// Subsystem: "successful",
			Name: "latency_seconds",
			Help: "System operations latency seconds",
		},
		[]string{"name", "tag"},
	)
)

func GetMetric(col prometheus.Collector) dto.Metric {
	c := make(chan prometheus.Metric, 1) // 1 for metric with no vector
	col.Collect(c)                       // collect current metric value into the channel
	m := dto.Metric{}
	_ = (<-c).Write(&m) // read metric value from the channel
	return m
}

func GetCounterValue(col prometheus.Collector) float64 {
	m := GetMetric(col)
	return *m.Counter.Value
}
