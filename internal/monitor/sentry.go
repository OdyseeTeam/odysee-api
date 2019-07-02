package monitor

import (
	"github.com/lbryio/lbrytv/config"

	"github.com/getsentry/sentry-go"
)

type VersionTag struct {
	LbrytvVersion string
	SDKVersion    string
}

func configureSentry(release, env string) {
	dsn := config.GetSentryDSN()
	if dsn == "" {
		return
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:         dsn,
		Release:     release,
		Environment: env,
	})
	if err != nil {
		Logger.Errorf("sentry initialization failed: %v", err)
	}
}

func SetVersionTag(tag VersionTag) {
	sentry.ConfigureScope(func(scope *sentry.Scope) {
		if tag.LbrytvVersion != "" {
			scope.SetTag("lbrytv_version", tag.LbrytvVersion)
		}
		if tag.LbrytvVersion != "" {
			scope.SetTag("lbrysdk_version", tag.SDKVersion)
		}
	})
}

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
