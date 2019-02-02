package monitor

import (
	"testing"

	"github.com/ybbus/jsonrpc"

	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func TestLogSuccessfulQuery(t *testing.T) {
	hook := test.NewLocal(Logger)
	LogSuccessfulQuery("account_balance", 0.025)
	assert.Equal(t, 1, len(hook.Entries))
	assert.Equal(t, log.InfoLevel, hook.LastEntry().Level)
	assert.Equal(t, "account_balance", hook.LastEntry().Data["method"])
	assert.Equal(t, 0.025, hook.LastEntry().Data["processing_time"])
	assert.Equal(t, "processed a call", hook.LastEntry().Message)
	hook.Reset()
}
func TestLogFailedQuery(t *testing.T) {
	hook := test.NewLocal(Logger)
	response := &jsonrpc.RPCError{
		Code: 111,
		// TODO: Uncomment after lbrynet 0.31 release
		// Message: "Invalid method requested: unknown_method.",
		Message: "Method Not Found",
	}
	queryParams := map[string]string{"param1": "value1"}
	LogFailedQuery("unknown_method", queryParams, response)
	assert.Equal(t, 1, len(hook.Entries))
	assert.Equal(t, log.ErrorLevel, hook.LastEntry().Level)
	assert.Equal(t, "unknown_method", hook.LastEntry().Data["method"])
	assert.Equal(t, queryParams, hook.LastEntry().Data["query"])
	assert.Equal(t, response, hook.LastEntry().Data["response"])
	assert.Equal(t, "server responded with error", hook.LastEntry().Message)
	hook.Reset()
}
