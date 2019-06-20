package monitor

import (
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/version"

	"github.com/getsentry/sentry-go"
)

type VersionTag struct {
	LbrytvVersion string
	SDKVersion    string
}

func configureSentry() {
	dsn := config.Settings.GetString("SentryDSN")
	if dsn == "" {
		return
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:     dsn,
		Release: version.GetDevVersion(),
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
