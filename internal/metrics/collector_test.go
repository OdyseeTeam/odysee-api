package metrics

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetMetricsValue(t *testing.T) {
	m := NewCollector()

	m.SetMetricsValue("resolve", 0.25, nil)
	assert.Equal(t, 0.25, math.Round(m.GetMetricsValue("resolve").Value*100)/100)

	m.SetMetricsValue("resolve", 0.1, nil)
	assert.Equal(t, 0.1, math.Round(m.GetMetricsValue("resolve").Value*100)/100)
}

func TestGetMetricsValue(t *testing.T) {
	m := NewCollector()

	assert.Equal(t, 0., m.GetMetricsValue("resolve").Value)
}

func TestMetricsIncrementDecrement(t *testing.T) {
	m := NewCollector()

	m.MetricsIncrement("players_count", 1)
	assert.Equal(t, 1.0, m.GetMetricsValue("players_count").Value)

	m.MetricsDecrement("players_count", 1)
	assert.Equal(t, 0.0, m.GetMetricsValue("players_count").Value)
}
