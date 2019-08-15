package metrics_server

import (
	"net/http"
	"runtime"
	"sync"

	"github.com/lbryio/lbrytv/app/proxy"
	"github.com/lbryio/lbrytv/internal/monitor"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	// "github.com/prometheus/client_golang/prometheus/promauto"
)

var once sync.Once

type Server struct {
	monitor.ModuleLogger

	proxy   *proxy.Service
	Address string
	Path    string
}

func NewServer(address string, path string, p *proxy.Service) *Server {
	return &Server{monitor.NewModuleLogger("metrics_server"), p, address, path}
}

func (s *Server) Serve() {
	once.Do(func() {
		s.registerMetrics()

		go func() {
			http.Handle(s.Path, promhttp.Handler())
			http.ListenAndServe(s.Address, nil)
		}()
		s.Log().Infof("metrics server listening on %v%v", s.Address, s.Path)
	})
}

func (s *Server) registerMetrics() {
	if err := prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "proxy",
			Name:      "resolve_time",
			Help:      "Time to serve a single resolve call.",
		},
		func() float64 { return s.proxy.GetExecTimeMetrics("resolve").ExecTime },
	)); err == nil {
		s.Log().Info("gauge 'proxy_resolve_time' registered")
	}

	if err := prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "proxy",
			Name:      "claim_search_time",
			Help:      "Time to serve a claim_search call.",
		},
		func() float64 { return s.proxy.GetExecTimeMetrics("claim_search").ExecTime },
	)); err == nil {
		s.Log().Info("gauge 'proxy_claim_search' registered")
	}

	if err := prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "player",
			Name:      "serving_streams_count",
			Help:      "Number of blob streams currently being served.",
		},
		func() float64 { return 0.0 },
	)); err == nil {
		s.Log().Info("gauge 'player_serving_streams_count' registered")
	}

	if err := prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "runtime",
			Name:      "goroutines_count",
			Help:      "Number of goroutines that currently exist.",
		},
		func() float64 { return float64(runtime.NumGoroutine()) },
	)); err == nil {
		s.Log().Info("gauge 'goroutines_count' registered")
	}
}
