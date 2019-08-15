// Package metrics is for maintaining async-safe dict of named values with extra associated with them parameters.
// A common example is SDK call execution times stored and accessed by method names.
package metrics

// Item is an object containing current value for a given metrics.
type Item struct {
	Name   string
	Value  float64
	Params interface{}
}

// Collector is a main object that stores the values and provides methods for setting and retrieving them,
type Collector struct {
	data       map[string]*Item
	operations chan op
	queries    chan metricsQuery
}

const opSet = 1
const opIncrement = 2
const opDecrement = 3

type op struct {
	kind uint
	item *Item
}

type metricsQuery struct {
	name            string
	responseChannel chan Item
}

// NewCollector creates and starts a single Collector instance.
// Instance goroutine is supposed to run thorough the whole lifetime of a program.
// Multiple independent Collector instances can be run.
func NewCollector() *Collector {
	metrics := Collector{
		data:       map[string]*Item{},
		operations: make(chan op),
		queries:    make(chan metricsQuery),
	}
	go func() {
		for {
			select {
			case query := <-metrics.queries:
				query.responseChannel <- *metrics.data[query.name]
			case m := <-metrics.operations:
				if metrics.data[m.item.Name] == nil {
					metrics.data[m.item.Name] = &Item{}
				}
				switch m.kind {
				case opSet:
					metrics.data[m.item.Name] = m.item
				case opIncrement:
					metrics.data[m.item.Name].Value += m.item.Value
				case opDecrement:
					metrics.data[m.item.Name].Value -= m.item.Value
				}
			}
		}
	}()
	return &metrics
}

// SetMetricsValue allows to set a named item value, including extra parameters,
// such as SDK call parameters.
func (m *Collector) SetMetricsValue(name string, time float64, params interface{}) {
	m.operations <- op{opSet, &Item{name, time, params}}
}

// GetMetricsValue returns a current value for a named parameter.
func (m *Collector) GetMetricsValue(name string) Item {
	response := make(chan Item)
	m.queries <- metricsQuery{name, response}
	return <-response
}

// MetricsIncrement increases a named metric for a given value.
// If a named metric does not exists, it is created and set to 0.
func (m *Collector) MetricsIncrement(name string, value float64) {
	m.operations <- op{opIncrement, &Item{Name: name, Value: value}}
}

// MetricsDecrement decreases a named metric for a given value.
// If a named metric does not exists, it is created and set to 0.
func (m *Collector) MetricsDecrement(name string, value float64) {
	m.operations <- op{opDecrement, &Item{Name: name, Value: value}}
}
