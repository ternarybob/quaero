package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Version information
var (
	Version   = "1.0.0"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// GetVersion returns the current version string
func GetVersion() string {
	return Version
}

// GetFullVersion returns version with build info
func GetFullVersion() string {
	return fmt.Sprintf("%s (build: %s, commit: %s)", Version, BuildTime, GitCommit)
}

// LoadVersionFromFile reads version from .version file if it exists
func LoadVersionFromFile() string {
	exePath, err := os.Executable()
	if err != nil {
		return Version
	}

	exeDir := filepath.Dir(exePath)
	versionFile := filepath.Join(exeDir, ".version")

	data, err := os.ReadFile(versionFile)
	if err != nil {
		return Version
	}

	version := strings.TrimSpace(string(data))
	if version != "" {
		Version = version
	}

	return Version
}
