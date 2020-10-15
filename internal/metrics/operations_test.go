package metrics

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOperation(t *testing.T) {
	op := StartOperation("db")
	op.AddTag("wallet")

	time.Sleep(20 * time.Millisecond)

	op.End()

	m := GetMetric(operations)
	assert.Equal(t, float64(0.02), math.Floor(m.Summary.GetSampleSum()*100)/100)
	assert.Equal(t, "name", *m.Label[0].Name)
	assert.Equal(t, "db", *m.Label[0].Value)
	assert.Equal(t, "tag", *m.Label[1].Name)
	assert.Equal(t, "wallet", *m.Label[1].Value)
}
