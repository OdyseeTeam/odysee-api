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

func StartOperation(name string) Operation {
	return Operation{started: time.Now(), name: name, labels: prometheus.Labels{"name": name}}
}

func (o *Operation) AddTag(value string) {
	o.labels["tag"] = value
}

func (o Operation) End() {
	o.duration = time.Since(o.started).Seconds()
	operations.With(o.labels).Observe(o.duration)
}
