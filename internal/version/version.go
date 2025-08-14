// Package version provides build-time version information for the weather service.
// It includes version numbers, build timestamps, git information, and runtime details
// that can be injected during the build process using ldflags.
package version

import (
	"runtime"
	"time"
)

// Build-time variables set via ldflags.
// These default values are used during development when not building with ldflags.
var (
	// Version is the current version of the application
	Version = "1.0.0"
	
	// BuildTime is when the binary was built (RFC3339 format)
	// Default to "unknown" to avoid empty string checks
	BuildTime = "unknown"
	
	// GitCommit is the git commit hash
	GitCommit = "unknown"
	
	// GitBranch is the git branch
	GitBranch = "unknown"
)

// Info contains version and build information.
type Info struct {
	Version   string    `json:"version"`
	BuildTime string    `json:"build_time"`
	GitCommit string    `json:"git_commit"`
	GitBranch string    `json:"git_branch"`
	GoVersion string    `json:"go_version"`
	Platform  string    `json:"platform"`
	BuildDate time.Time `json:"build_date"`
}

// Get returns version and build information.
//
// Returns:
//   - Info: Version details including Go runtime and platform information
func Get() Info {
	var buildDate time.Time
	
	// Attempt to parse BuildTime as RFC3339 timestamp
	// This will fail for "unknown" (development) and succeed for actual timestamps (production)
	if t, err := time.Parse(time.RFC3339, BuildTime); err == nil {
		buildDate = t
	}

	return Info{
		Version:   Version,
		BuildTime: BuildTime,
		GitCommit: GitCommit,
		GitBranch: GitBranch,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
		BuildDate: buildDate,
	}
}