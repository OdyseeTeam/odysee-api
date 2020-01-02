package metrics

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Timer struct {
	Started  time.Time
	Duration float64
	hist     prometheus.Histogram
}

func TimerStart() *Timer {
	return &Timer{Started: time.Now()}
}

func (t *Timer) Observe(hist prometheus.Histogram) *Timer {
	t.hist = hist
	return t
}

func (t *Timer) Done() float64 {
	if t.Duration == 0 {
		t.Duration = time.Since(t.Started).Seconds()
		if t.hist != nil {
			t.hist.Observe(t.Duration)
		}
	}
	return t.Duration
}

func (t *Timer) String() string {
	return fmt.Sprintf("%.2f", t.Duration)
}
