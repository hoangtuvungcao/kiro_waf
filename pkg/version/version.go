// Package version provides build-time version information shared across
// all Kiro WAF binaries (master, client, CLI).
//
// Version variables are set at build time via -ldflags:
//
//	go build -ldflags "-X kiro_waf/pkg/version.Version=1.0.0 -X kiro_waf/pkg/version.Commit=abc123 -X kiro_waf/pkg/version.BuildDate=2024-01-01T00:00:00Z"
package version

import "fmt"

// Build-time variables set via -ldflags.
var (
	// Version is the semantic version of the binary (e.g. "1.0.0").
	Version = "0.1.0-dev"

	// Commit is the git commit hash at build time.
	Commit = "unknown"

	// BuildDate is the ISO 8601 date when the binary was built.
	BuildDate = "unknown"
)

// Info returns a formatted version string including version, commit, and build date.
func Info() string {
	return fmt.Sprintf("kiro-waf %s (commit: %s, built: %s)", Version, Commit, BuildDate)
}

// Short returns just the version string.
func Short() string {
	return Version
}
