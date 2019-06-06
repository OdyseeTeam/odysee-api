package monitor

import (
	"testing"

	"github.com/lbryio/lbrytv/config"

	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

func TestLogSuccessfulQuery(t *testing.T) {
	hook := test.NewLocal(Logger)

	LogSuccessfulQuery("account_balance", 0.025)

	require.Equal(t, 1, len(hook.Entries))
	require.Equal(t, log.InfoLevel, hook.LastEntry().Level)
	require.Equal(t, "account_balance", hook.LastEntry().Data["method"])
	require.Equal(t, 0.025, hook.LastEntry().Data["processing_time"])
	require.Equal(t, "processed a call", hook.LastEntry().Message)

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

	require.Equal(t, 1, len(hook.Entries))
	require.Equal(t, log.ErrorLevel, hook.LastEntry().Level)
	require.Equal(t, "unknown_method", hook.LastEntry().Data["method"])
	require.Equal(t, queryParams, hook.LastEntry().Data["query"])
	require.Equal(t, response, hook.LastEntry().Data["response"])
	require.Equal(t, "server responded with error", hook.LastEntry().Message)

	hook.Reset()
}

func TestModuleLoggerLogF(t *testing.T) {
	hook := test.NewLocal(Logger)

	l := NewModuleLogger("db")
	l.LogF(F{"number": 1}).Info("error!")
	require.Equal(t, 1, len(hook.Entries))
	require.Equal(t, log.InfoLevel, hook.LastEntry().Level)
	require.Equal(t, 1, hook.LastEntry().Data["number"])
	require.Equal(t, "db", hook.LastEntry().Data["module"])
	require.Equal(t, "error!", hook.LastEntry().Message)

	hook.Reset()
}

func TestModuleLoggerLog(t *testing.T) {
	hook := test.NewLocal(Logger)

	l := NewModuleLogger("db")
	l.Log().Info("error!")
	require.Equal(t, 1, len(hook.Entries))
	require.Equal(t, log.InfoLevel, hook.LastEntry().Level)
	require.Equal(t, "db", hook.LastEntry().Data["module"])
	require.Equal(t, "error!", hook.LastEntry().Message)

	hook.Reset()
}

func TestModuleLoggerLogF_LogTokensDisabled(t *testing.T) {
	hook := test.NewLocal(Logger)

	config.Override("Debug", 0)
	defer config.RestoreOverridden()

	l := NewModuleLogger("auth")
	l.LogF(F{"token": "secret", "email": "abc@abc.com"}).Info("something happened")
	require.Equal(t, "abc@abc.com", hook.LastEntry().Data["email"])
	require.Equal(t, masked, hook.LastEntry().Data["token"])

	hook.Reset()
}
