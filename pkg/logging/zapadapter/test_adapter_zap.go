package zapadapter

import (
	"testing"

	"logur.dev/logur"
	"logur.dev/logur/logtesting"
)

func TestKVLogger(t *testing.T) {
	logger := &logur.TestLogger{}
	log := NewKV(nil)
	log.Info("hello log", "abc", 123, "foo", "bar")

	logEvent := logur.LogEvent{
		Line:  "hello log",
		Level: logur.Info,
		Fields: map[string]interface{}{
			"foo": "bar",
			"abc": "123",
		},
	}

	logtesting.AssertLogEventsEqual(t, logEvent, *(logger.LastEvent()))
}
