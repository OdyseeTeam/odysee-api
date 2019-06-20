package monitor

import (
	"fmt"

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
		fmt.Printf("sentry initialization failed: %v", err)
	}

	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag("lbrytv_version", "1.1.1.1")
		scope.SetTag("lbrynet_version", "1.1.1.1")
	})
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
