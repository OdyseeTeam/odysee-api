package monitor

import (
	"testing"

	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc/v2"
)

func TestLogSuccessfulQuery(t *testing.T) {
	hook := test.NewLocal(logger.Entry.Logger)

	config.Override("ShouldLogResponses", false)
	defer config.RestoreOverridden()

	response := &jsonrpc.RPCResponse{
		Result: map[string]interface{}{
			"available": "20.02",
			"reserved":  "0.0",
			"reserved_subtotals": map[string]string{
				"claims":   "0.0",
				"supports": "0.0",
				"tips":     "0.0",
			},
			"total": "20.02",
		},
	}

	LogSuccessfulQuery("resolve", 0.025, map[string]string{"urls": "one"}, response)

	require.Equal(t, 1, len(hook.Entries))
	assert.Equal(t, logrus.InfoLevel, hook.LastEntry().Level)
	assert.Equal(t, "resolve", hook.LastEntry().Data["method"])
	assert.Equal(t, map[string]string{"urls": "one"}, hook.LastEntry().Data["params"])
	assert.Equal(t, "0.025", hook.LastEntry().Data["duration"])
	assert.Equal(t, "call processed", hook.LastEntry().Message)

	LogSuccessfulQuery("account_balance", 0.025, nil, nil)

	require.Equal(t, 2, len(hook.Entries))
	assert.Equal(t, logrus.InfoLevel, hook.LastEntry().Level)
	assert.Equal(t, "account_balance", hook.LastEntry().Data["method"])
	assert.Equal(t, nil, hook.LastEntry().Data["params"])
	assert.Equal(t, "0.025", hook.LastEntry().Data["duration"])
	assert.Nil(t, hook.LastEntry().Data["response"])
	assert.Equal(t, "call processed", hook.LastEntry().Message)

	hook.Reset()
}

func TestModuleLoggerLogF(t *testing.T) {
	l := NewModuleLogger("storage")
	hook := test.NewLocal(l.Entry.Logger)
	l.WithFields(logrus.Fields{"number": 1}).Info("error!")

	require.Equal(t, 1, len(hook.Entries))
	assert.Equal(t, logrus.InfoLevel, hook.LastEntry().Level)
	assert.Equal(t, 1, hook.LastEntry().Data["number"])
	assert.Equal(t, "storage", hook.LastEntry().Data["module"])
	assert.Equal(t, "error!", hook.LastEntry().Message)

	hook.Reset()
}

func TestModuleLoggerLog(t *testing.T) {
	l := NewModuleLogger("storage")
	hook := test.NewLocal(l.Entry.Logger)
	l.Log().Info("error!")

	require.Equal(t, 1, len(hook.Entries))
	assert.Equal(t, logrus.InfoLevel, hook.LastEntry().Level)
	assert.Equal(t, "storage", hook.LastEntry().Data["module"])
	assert.Equal(t, "error!", hook.LastEntry().Message)

	hook.Reset()
}

func TestModuleLoggerMasksTokens(t *testing.T) {
	l := NewModuleLogger("auth")
	hook := test.NewLocal(l.Entry.Logger)

	config.Override("Debug", false)
	defer config.RestoreOverridden()

	l.WithFields(logrus.Fields{"token": "SecRetT0Ken", "email": "abc@abc.com"}).Info("something happened")
	assert.Equal(t, "abc@abc.com", hook.LastEntry().Data["email"])
	assert.Equal(t, valueMask, hook.LastEntry().Data["token"])

	hook.Reset()
}
