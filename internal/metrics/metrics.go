package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	nsPlayer  = "player"
	nsIAPI    = "iapi"
	nsProxy   = "proxy"
	nsLbrynet = "lbrynet"
	nsUI      = "ui"

	LabelSource   = "source"
	LabelInstance = "instance"
)

var (
	PlayerStreamsRunning = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: nsPlayer,
		Subsystem: "streams",
		Name:      "running",
		Help:      "Number of streams currently playing",
	})
	PlayerRetrieverSpeed = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: nsPlayer,
		Subsystem: "retriever",
		Name:      "speed_mbps",
		Help:      "Speed of blob/chunk retrieval",
	}, []string{LabelSource})

	PlayerInBytes = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: nsPlayer,
		Name:      "in_bytes",
		Help:      "Total number of bytes downloaded",
	})
	PlayerOutBytes = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: nsPlayer,
		Name:      "out_bytes",
		Help:      "Total number of bytes streamed out",
	})

	PlayerCacheHitCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: nsPlayer,
		Subsystem: "cache",
		Name:      "hit_count",
		Help:      "Total number of blobs found in the local cache",
	})
	PlayerCacheMissCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: nsPlayer,
		Subsystem: "cache",
		Name:      "miss_count",
		Help:      "Total number of blobs that were not in the local cache",
	})
	PlayerCacheErrorCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: nsPlayer,
		Subsystem: "cache",
		Name:      "error_count",
		Help:      "Total number of errors retrieving blobs from the local cache",
	})

	PlayerCacheSize = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: nsPlayer,
		Subsystem: "cache",
		Name:      "size",
		Help:      "Current size of cache",
	})
	PlayerCacheDroppedCount = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: nsPlayer,
		Subsystem: "cache",
		Name:      "dropped_count",
		Help:      "Total number of blobs dropped at the admission time",
	})
	PlayerCacheRejectedCount = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: nsPlayer,
		Subsystem: "cache",
		Name:      "rejected_count",
		Help:      "Total number of blobs rejected at the admission time",
	})

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

	callsSecondsBuckets = []float64{0.005, 0.025, 0.05, 0.1, 0.25, 0.4, 1, 2, 5, 10, 20, 60, 120, 300}

	ProxyCallDurations = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: nsProxy,
			Subsystem: "calls",
			Name:      "total_seconds",
			Help:      "Method call latency distributions",
			Buckets:   callsSecondsBuckets,
		},
		[]string{"method", "endpoint"},
	)
	ProxyCallFailedDurations = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: nsProxy,
			Subsystem: "calls",
			Name:      "failed_seconds",
			Help:      "Failed method call latency distributions",
			Buckets:   callsSecondsBuckets,
		},
		[]string{"method", "endpoint"},
	)

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
)
