package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Timer struct {
	Started  time.Time
	Duration float64
	hist     prometheus.Histogram
}

func TimerStart(hist prometheus.Histogram) *Timer {
	return &Timer{Started: time.Now(), hist: hist}
}

func (t *Timer) Done() float64 {
	if t.Duration == 0 {
		t.Duration = time.Since(t.Started).Seconds()
		t.hist.Observe(t.Duration)
	}
	return t.Duration
}
