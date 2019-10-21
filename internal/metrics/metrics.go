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
		Name: "player_streams_running",
		Help: "Number of streams currently playing",
	})
	PlayerResponseSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: nsPlayer,
		Subsystem: "response",
		Name:      "success_seconds",
		Help:      "Time to successful response",
	})
	PlayerFailureSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: nsPlayer,
		Subsystem: "failure_response",
		Name:      "seconds",
		Help:      "Time to failed response",
	})

	IAPIAuthSuccessSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: nsIAPI,
		Subsystem: "auth",
		Name:      "success_seconds",
		Help:      "Time to successful authentication",
	})
	IAPIAuthFailureSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: nsIAPI,
		Subsystem: "auth",
		Name:      "failure_seconds",
		Help:      "Time to failed authentication response",
	})

	ProxyCallDurations = promauto.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace:  nsProxy,
			Subsystem:  "calls_durations",
			Name:       "seconds",
			Help:       "Method call latency distributions",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"method"},
	)
	ProxyCallFailureDurations = promauto.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace:  nsProxy,
			Subsystem:  "failed_calls_durations",
			Name:       "seconds",
			Help:       "Failed method call latency distributions",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"method"},
	)
)
