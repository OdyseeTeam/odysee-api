package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Operation struct {
	started  time.Time
	duration float64
	name     string
	labels   prometheus.Labels
}

func StartOperation(name, tag string) Operation {
	return Operation{started: time.Now(), name: name, labels: prometheus.Labels{"name": name, "tag": tag}}
}

func (o Operation) DurationSeconds() float64 {
	return time.Since(o.started).Seconds()
}

func (o Operation) End() {
	o.duration = time.Since(o.started).Seconds()
	operations.With(o.labels).Observe(o.duration)
}
