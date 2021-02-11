package metrics

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Timer struct {
	Started   time.Time
	duration  float64
	observers []prometheus.Observer
}

func StartTimer() *Timer {
	return &Timer{Started: time.Now()}
}

func (t *Timer) AddObserver(o prometheus.Observer) {
	t.observers = append(t.observers, o)
}

func (t *Timer) Stop() float64 {
	if t.duration == 0 {
		t.duration = time.Since(t.Started).Seconds()
		for _, o := range t.observers {
			o.Observe(t.duration)
		}
	}
	return t.duration
}

func (t *Timer) Duration() float64 {
	if t.duration == 0 {
		return time.Since(t.Started).Seconds()
	}
	return t.duration
}

func (t *Timer) String() string {
	return fmt.Sprintf("%.2f", t.Duration())
}
