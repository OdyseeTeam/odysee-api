package logging

type TracedObject interface {
	GetTraceData() map[string]string
}

func TracedLogger(l KVLogger, t TracedObject) KVLogger {
	for k, v := range t.GetTraceData() {
		l = l.With(k, v)
	}
	return l
}
