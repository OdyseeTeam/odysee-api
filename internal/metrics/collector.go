package metrics

type MetricsItem struct {
	Name   string
	Value  float64
	Params interface{}
}

type MetricsValues struct {
	data      map[string]MetricsItem
	collector chan MetricsItem
	queries   chan metricsQuery
}

type metricsQuery struct {
	name            string
	responseChannel chan MetricsItem
}

func NewMetrics() *MetricsValues {
	metrics := MetricsValues{
		map[string]MetricsItem{},
		make(chan MetricsItem),
		make(chan metricsQuery),
	}
	go func() {
		for {
			select {
			case query := <-metrics.queries:
				query.responseChannel <- metrics.data[query.name]
			case m := <-metrics.collector:
				metrics.data[m.Name] = m
			}
		}
	}()
	return &metrics
}

func (m *MetricsValues) SetMetricsValue(name string, time float64, params interface{}) {
	m.collector <- MetricsItem{name, time, params}
}

func (m *MetricsValues) GetMetricsValue(name string) MetricsItem {
	response := make(chan MetricsItem)
	m.queries <- metricsQuery{name, response}
	return <-response
}
