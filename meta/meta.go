package meta

import (
	"os/exec"
	"strings"
)

var Debugging bool

// version and commitMsg get filled in using -ldflags when the binary gets built
var version string
var commitMsg string

func GetVersion() string {
	if version != "" {
		return version
	}

	if Debugging {
		out, err := exec.Command("git", "describe", "--always", "--dirty", "--long").Output()
		if err != nil {
			return err.Error()
		}
		return strings.TrimSpace(string(out))

	}

	return "unknown"
}

func GetCommitMessage() string {
	if commitMsg != "" {
		return commitMsg
	}

	if Debugging {
		out, err := exec.Command("git", "show", "-s", "--format=%s", "HEAD").Output()
		if err != nil {
			return err.Error()
		}
		return strings.TrimSpace(string(out))

	}

	return "unknown"
}
