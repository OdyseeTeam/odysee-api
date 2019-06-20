package version

import "fmt"

var version string
var commit string
var date string

// GetVersion returns current application version
func GetVersion() string {
	return version
}

// GetDevVersion returns current app version plus commit
func GetDevVersion() string {
	if commit != "" {
		return fmt.Sprintf("%v-%v", GetVersion(), commit[:6])
	}
	return "unknown"
}

// GetFullBuildName returns current app version, commit and build time
func GetFullBuildName() string {
	return fmt.Sprintf("%v, commit %v, built at %v", GetVersion(), commit, date)
}
