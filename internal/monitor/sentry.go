package monitor

import (
	"github.com/OdyseeTeam/odysee-api/internal/responses"

	"github.com/getsentry/sentry-go"
)

var ignored = []string{
	responses.AuthRequiredErrorMessage,
}

func ConfigureSentry(dsn, release, env string) {
	if dsn == "" {
		logger.Log().Info("sentry disabled (no DSN configured)")
		return
	}

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Release:          release,
		Environment:      env,
		AttachStacktrace: true,
		IgnoreErrors:     ignored,
		// TracesSampleRate: .5,
	}); err != nil {
		logger.Log().Warnf("sentry initialization failed: %v", err)
		return
	}
	logger.Log().Info("sentry initialized")
}

// ErrorToSentry sends to Sentry general exception info with some optional extra detail (like user email, claim url etc)
func ErrorToSentry(err error, params ...map[string]string) *sentry.EventID {
	var extra map[string]string
	var eventID *sentry.EventID
	if len(params) > 0 {
		extra = params[0]
	} else {
		extra = map[string]string{}
	}

	sentry.WithScope(func(scope *sentry.Scope) {
		for k, v := range extra {
			scope.SetExtra(k, v)
		}
		sentry.CaptureException(err)
	})
	return eventID
}

func MessageToSentry(msg string, level sentry.Level, params map[string]string) *sentry.EventID {
	var eventID *sentry.EventID
	sentry.WithScope(func(scope *sentry.Scope) {
		for k, v := range params {
			scope.SetExtra(k, v)
		}
		event := sentry.NewEvent()
		event.Level = level
		event.Message = msg
		eventID = sentry.CaptureEvent(event)
	})
	return eventID
}
