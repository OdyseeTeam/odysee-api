package metrics

type ExecTimeMetricsItem struct {
	Name     string
	ExecTime float64
	Params   interface{}
}

type ExecTimeMetrics struct {
	data      map[string]ExecTimeMetricsItem
	collector chan ExecTimeMetricsItem
	queries   chan metricsQuery
}

type metricsQuery struct {
	name            string
	responseChannel chan ExecTimeMetricsItem
}

func NewMetrics() *ExecTimeMetrics {
	metrics := ExecTimeMetrics{
		map[string]ExecTimeMetricsItem{},
		make(chan ExecTimeMetricsItem),
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

func (m *ExecTimeMetrics) LogExecTime(name string, time float64, params interface{}) {
	m.collector <- ExecTimeMetricsItem{name, time, params}
}

func (m *ExecTimeMetrics) GetExecTimeMetrics(name string) ExecTimeMetricsItem {
	response := make(chan ExecTimeMetricsItem)
	m.queries <- metricsQuery{name, response}
	return <-response
}
