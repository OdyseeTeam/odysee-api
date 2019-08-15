package metrics

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	m.LogExecTime("resolve", 0.25, nil)
	assert.Equal(t, 0.25, math.Round(m.GetExecTimeMetrics("resolve").ExecTime*100)/100)
}
