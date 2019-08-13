package metrics_server

import (
	"fmt"
	"net/http"
	"runtime"
	"sync"

	"github.com/lbryio/lbrytv/app/proxy"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	// "github.com/prometheus/client_golang/prometheus/promauto"
)

var once sync.Once

type Server struct {
	proxy *proxy.Service
}

func NewServer(p *proxy.Service) *Server {
	return &Server{p}
}

func (s *Server) Serve() {
	once.Do(func() {
		s.registerMetrics()

		go func() {
			http.Handle("/metrics", promhttp.Handler())
			http.ListenAndServe(":2112", nil)
		}()
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
		fmt.Println("GaugeFunc 'proxy_resolve_time' registered.")
	}

	if err := prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "player",
			Name:      "serving_streams_count",
			Help:      "Number of blob streams currently being served.",
		},
		func() float64 { return 0.0 },
	)); err == nil {
		fmt.Println("GaugeFunc 'player_serving_streams_count' registered.")
	}

	if err := prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: "runtime",
			Name:      "goroutines_count",
			Help:      "Number of goroutines that currently exist.",
		},
		func() float64 { return float64(runtime.NumGoroutine()) },
	)); err == nil {
		fmt.Println("GaugeFunc 'goroutines_count' registered.")
	}
}
