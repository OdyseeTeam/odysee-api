package metrics_server

import (
	"fmt"
	"net/http"
	"runtime"
	"sync"

	"github.com/lbryio/lbrytv/api"
	"github.com/lbryio/lbrytv/app/proxy"
	"github.com/lbryio/lbrytv/internal/monitor"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	// "github.com/prometheus/client_golang/prometheus/promauto"
)

var once sync.Once

var monitoredProxyCalls = []string{
	proxy.MethodClaimSearch,
	proxy.MethodResolve,
	proxy.MethodAccountBalance,
	proxy.MethodAccountList,
	proxy.MethodGet,
	proxy.MethodFileList,
}

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
	for _, m := range monitoredProxyCalls {
		m := m
		if err := prometheus.Register(prometheus.NewGaugeFunc(
			prometheus.GaugeOpts{
				Subsystem: "proxy",
				Name:      fmt.Sprintf("%v_time", m),
				Help:      "Time to serve a single resolve call.",
			},
			func() float64 { return s.proxy.GetMetricsValue(m).Value },
		)); err == nil {
			s.Log().Infof("gauge 'proxy_%v_time' registered", m)
		}
	}

	if err := prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "player",
			Name:      "instances_count",
			Help:      "Number of blob streams currently being served.",
		},
		func() float64 { return api.Collector.GetMetricsValue("player_instances_count").Value },
	)); err == nil {
		s.Log().Info("gauge 'player_instances_count' registered")
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
