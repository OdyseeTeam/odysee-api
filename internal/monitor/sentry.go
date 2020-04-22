package monitor

import (
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/responses"

	"github.com/getsentry/sentry-go"
)

var IgnoredExceptions = []string{
	responses.AuthRequiredErrorMessage,
}

func configureSentry(release, env string) {
	dsn := config.GetSentryDSN()
	if dsn == "" {
		logger.Log().Info("sentry disabled (no DNS configured)")
		return
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Release:          release,
		Environment:      env,
		AttachStacktrace: true,
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			if len(event.Exception) > 0 {
				for _, ie := range IgnoredExceptions {
					if event.Exception[0].Value == ie {
						return nil
					}
				}
			}
			return event
		},
	})
	if err != nil {
		logger.Log().Errorf("sentry initialization failed: %v", err)
	} else {
		logger.Log().Info("sentry initialized")
	}
}

// ErrorToSentry sends to Sentry general exception info with some extra provided detail (like user email, claim url etc)
func ErrorToSentry(err error, params ...map[string]string) {
	var extra map[string]string
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
}
