package proxy

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestForwardCall_ForbiddenMethod(t *testing.T) {
	for _, m := range forbiddenMethods {
		r := call(t, m)
		assert.Equal(t, fmt.Sprintf("Forbidden method requested: %s", m), r.Error.Message)
		assert.Equal(t, -32601, r.Error.Code)
	}
}

func TestForwardCall_ForbiddenParamAccountID(t *testing.T) {
	r := call(t, "transaction_list", map[string]string{"account_id": "abcdef"})
	assert.Equal(t, "Forbidden parameter supplied: account_id", r.Error.Message)
	assert.Equal(t, -32602, r.Error.Code)
}
