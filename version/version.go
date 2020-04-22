package version

import "fmt"

var (
	version   = "unknown"
	commit    = "unknown"
	buildDate = "unknown"
)

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

func BuildInfo() map[string]interface{} {
	return map[string]interface{}{
		"buildVersion": GetVersion(),
		"buildCommit":  commit,
		"buildDate":    buildDate,
	}
}
