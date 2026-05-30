// Package install provides Go implementations of the install script logic
// for testability. These functions mirror the bash script's detect_os() and
// idempotency logic so they can be verified with property-based tests.
package install

import (
	"errors"
	"strings"
)

// OSInfo holds the detected operating system information.
type OSInfo struct {
	Distro     string // Canonical name: Ubuntu, Debian, CentOS, Rocky, Fedora, Arch
	Version    string // VERSION_ID from os-release (e.g., "22.04", "12", "9")
	PkgManager string // Package manager: apt, yum, dnf, pacman
}

// SupportedDistros lists all supported distribution identifiers.
var SupportedDistros = []string{"Ubuntu", "Debian", "CentOS", "Rocky", "Fedora", "Arch"}

// ErrUnsupportedOS is returned when the OS is not in the supported list.
var ErrUnsupportedOS = errors.New("unsupported operating system")

// ErrMissingID is returned when the os-release content has no ID field.
var ErrMissingID = errors.New("ID field missing in os-release")

// DetectOS parses the content of /etc/os-release and returns OS information.
// This mirrors the bash script's detect_os() function logic.
func DetectOS(content string) (OSInfo, error) {
	values := parseOSRelease(content)

	id := values["ID"]
	if id == "" {
		return OSInfo{}, ErrMissingID
	}

	version := values["VERSION_ID"]
	if version == "" {
		version = "unknown"
	}

	var distro, pkgManager string

	switch strings.ToLower(id) {
	case "ubuntu":
		distro = "Ubuntu"
		pkgManager = "apt"
	case "debian":
		distro = "Debian"
		pkgManager = "apt"
	case "centos":
		distro = "CentOS"
		pkgManager = "yum"
	case "rocky":
		distro = "Rocky"
		pkgManager = "dnf"
	case "fedora":
		distro = "Fedora"
		pkgManager = "dnf"
	case "arch":
		distro = "Arch"
		pkgManager = "pacman"
	default:
		return OSInfo{}, ErrUnsupportedOS
	}

	return OSInfo{
		Distro:     distro,
		Version:    version,
		PkgManager: pkgManager,
	}, nil
}

// parseOSRelease parses key=value pairs from os-release content.
// It handles quoted values and comments, matching the bash `source` behavior.
func parseOSRelease(content string) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		// Remove surrounding quotes (single or double)
		value = strings.Trim(value, `"'`)
		values[key] = value
	}
	return values
}

// InstallState represents the state of an installed Client_Node for idempotency checks.
type InstallState struct {
	BinaryExists   bool
	BinaryVersion  string
	ConfigExists   bool
	ServiceEnabled bool
	ServiceRunning bool
}

// InstallDecision represents what the install script should do given current state.
type InstallDecision struct {
	ShouldDownload    bool // Whether to download a new binary
	ShouldStopService bool // Whether to stop the service before replacement
	PreserveConfig    bool // Whether to preserve existing config
	EnsureEnabled     bool // Whether to ensure service is enabled
	EnsureRunning     bool // Whether to ensure service is running
}

// DecideInstallAction determines what actions the install script should take
// given the current system state and the target version. This mirrors the
// idempotency logic in the bash script.
func DecideInstallAction(state InstallState, targetVersion string) InstallDecision {
	decision := InstallDecision{
		PreserveConfig: state.ConfigExists, // Always preserve existing config
		EnsureEnabled:  true,               // Always ensure service is enabled
		EnsureRunning:  true,               // Always ensure service is running
	}

	if !state.BinaryExists {
		// Fresh install: download binary
		decision.ShouldDownload = true
		decision.ShouldStopService = false
		return decision
	}

	if state.BinaryVersion == targetVersion {
		// Same version: skip download, don't touch binary
		decision.ShouldDownload = false
		decision.ShouldStopService = false
		return decision
	}

	// Different version: stop service, download new binary
	decision.ShouldDownload = true
	decision.ShouldStopService = state.ServiceRunning
	return decision
}
