package monitor

import (
	"fmt"

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
		Logger.Errorf("sentry initialization failed: %v", err)
	} else {
		Logger.Info("Sentry initialized")
	}
}

// CaptureException sends to Sentry general exception info with some extra provided detail (like user email, claim url etc)
func CaptureException(err error, params ...map[string]string) {
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

// CaptureFailedQuery sends to Sentry details of a failed daemon call.
func CaptureFailedQuery(method string, query interface{}, errorResponse interface{}) {
	CaptureException(
		fmt.Errorf("daemon responded with an error when calling method %v", method),
		map[string]string{
			"method":   method,
			"query":    fmt.Sprintf("%v", query),
			"response": fmt.Sprintf("%v", errorResponse),
		},
	)
}
