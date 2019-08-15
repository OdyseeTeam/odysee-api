package metrics

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetMetricsValue(t *testing.T) {
	m := NewMetrics()

	m.SetMetricsValue("resolve", 0.25, nil)
	assert.Equal(t, 0.25, math.Round(m.GetMetricsValue("resolve").Value*100)/100)

	m.SetMetricsValue("resolve", 0.1, nil)
	assert.Equal(t, 0.1, math.Round(m.GetMetricsValue("resolve").Value*100)/100)
}
