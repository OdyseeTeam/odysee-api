package metrics

import (
	"net/http"
	"sync"

	"github.com/lbryio/lbrytv/internal/monitor"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var once sync.Once

type Server struct {
	monitor.ModuleLogger

	Address string
	Path    string
}

func NewServer(address string, path string) *Server {
	return &Server{monitor.NewModuleLogger("metrics"), address, path}
}

func (s *Server) Serve() {
	go func() {
		http.Handle(s.Path, promhttp.Handler())
		http.ListenAndServe(s.Address, nil)
	}()
	s.Log().Infof("metrics server listening on %v%v", s.Address, s.Path)
}

const (
	nsPlayer = "player"
	nsIAPI   = "iapi"
	nsProxy  = "proxy"

	LabelSource = "source"
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

	callsSecondsBuckets = []float64{0.02, 0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10}

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
)
