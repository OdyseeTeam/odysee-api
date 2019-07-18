package monitor

import (
	"fmt"

	"github.com/lbryio/lbrytv/config"

	"github.com/getsentry/sentry-go"
)

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
	})
	if err != nil {
		Logger.Errorf("sentry initialization failed: %v", err)
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

	sentry.ConfigureScope(func(scope *sentry.Scope) {
		for k, v := range extra {
			scope.SetExtra(k, v)
		}
		sentry.CaptureException(err)
	})
}

// captureFailedQuery sends to Sentry details of a failed daemon call.
func captureFailedQuery(method string, query interface{}, errorResponse interface{}) {
	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetExtra("method", method)
		scope.SetExtra("query", fmt.Sprintf("%v", query))
		scope.SetExtra("response", fmt.Sprintf("%v", errorResponse))
		sentry.CaptureMessage("Daemon responded with an error")
	})
}
