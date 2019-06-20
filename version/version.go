package version

import "fmt"

var (
	version = "unknown"
	commit  = "unknown"
	date    = "unknown"
)

var appName = "lbrytv"

// GetAppName returns main application name
func GetAppName() string {
	return appName
}

// GetVersion returns current application version
func GetVersion() string {
	return version
}

// GetDevVersion returns current app version plus commit
func GetDevVersion() string {
	if commit != "" {
		return fmt.Sprintf("%v-%v", GetVersion(), commit)
	}
	return "unknown"
}

// GetFullBuildName returns current app version, commit and build time
func GetFullBuildName() string {
	return fmt.Sprintf("%v %v, commit %v, built at %v", GetAppName(), GetVersion(), commit, date)
}
