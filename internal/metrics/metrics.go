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

const nsPlayer = "player"
const nsIAPI = "iapi"
const nsProxy = "proxy"

var (
	PlayerStreamsRunning = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: nsPlayer,
		Subsystem: "streams",
		Name:      "running",
		Help:      "Number of streams currently playing",
	})
	PlayerBlobDownloadDurations = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: nsPlayer,
		Subsystem: "blob",
		Name:      "download_seconds",
		Help:      "Blob download durations",
		Buckets:   []float64{0.1, 0.3, 0.6, 1.2},
	})
	PlayerBlobDeliveryDurations = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: nsPlayer,
		Subsystem: "blob",
		Name:      "delivery_seconds",
		Help:      "Blob delivery durations",
		Buckets:   []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
	})

	PlayerSuccessesCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: nsPlayer,
		Name:      "successes_total",
		Help:      "Total number of successfully loaded blobs",
	})
	PlayerFailuresCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: nsPlayer,
		Name:      "failures_total",
		Help:      "Total number of errors getting blobs",
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

	ProxyCallDurations = promauto.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace:  nsProxy,
			Subsystem:  "calls",
			Name:       "total_seconds",
			Help:       "Method call latency distributions",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"method"},
	)
	ProxyCallFailedDurations = promauto.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace:  nsProxy,
			Subsystem:  "calls",
			Name:       "failed_seconds",
			Help:       "Failed method call latency distributions",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"method"},
	)
	ProxyCallHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: nsProxy,
		Subsystem: "calls",
		Name:      "histogram_seconds",
		Help:      "Method calls latency histogram",
		Buckets:   []float64{0.01, 0.05, 0.1, 0.3, 0.6, 1.2, 3.0},
	})
)
