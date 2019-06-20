package monitor

import (
	"fmt"
	"github.com/lbryio/lbrytv/config"

	"github.com/getsentry/sentry-go"
)

type VersionTag struct {
	LbrytvVersion string
	SDKVersion    string
}

func configureSentry(release, env string) {
	dsn := config.Settings.GetString("SentryDSN")
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

func CaptureException(err error) {
	sentry.CaptureException(err)
}
