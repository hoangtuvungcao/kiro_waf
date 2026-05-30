// Feature: waf-system-overhaul, Property 13: OS Detection Correctness
// **Validates: Requirements 4.1**
//
// For any valid /etc/os-release file content containing a supported distribution
// identifier (Ubuntu, Debian, CentOS, Rocky, Fedora, Arch), the detection function
// SHALL correctly extract the distribution name and version number.
//
// Feature: waf-system-overhaul, Property 12: Install Script Idempotency
// **Validates: Requirements 4.9**
//
// For any system state where Client_Node is already installed, running the install
// script again with the same version SHALL not modify the binary, SHALL preserve
// existing configuration files, and SHALL leave the systemd service in enabled+running state.
package property

import (
	"fmt"
	"strings"
	"testing"

	"kiro_waf/internal/client/install"

	"pgregory.net/rapid"
)

// --- Property 13: OS Detection Correctness ---

// distroInfo maps supported distro IDs to their expected canonical names and package managers.
var distroInfo = map[string]struct {
	Name       string
	PkgManager string
}{
	"ubuntu": {Name: "Ubuntu", PkgManager: "apt"},
	"debian": {Name: "Debian", PkgManager: "apt"},
	"centos": {Name: "CentOS", PkgManager: "yum"},
	"rocky":  {Name: "Rocky", PkgManager: "dnf"},
	"fedora": {Name: "Fedora", PkgManager: "dnf"},
	"arch":   {Name: "Arch", PkgManager: "pacman"},
}

// supportedIDs is the list of supported distro IDs for generation.
var supportedIDs = []string{"ubuntu", "debian", "centos", "rocky", "fedora", "arch"}

// genOSReleaseContent generates a valid /etc/os-release content string for a supported distro.
func genOSReleaseContent(t *rapid.T) (string, string, string) {
	// Pick a random supported distro ID
	id := rapid.SampledFrom(supportedIDs).Draw(t, "distro_id")

	// Generate a random version string (e.g., "22.04", "12", "9.3", "39", "rolling")
	version := rapid.OneOf(
		rapid.Just("22.04"),
		rapid.Just("24.04"),
		rapid.Just("12"),
		rapid.Just("11"),
		rapid.Just("9"),
		rapid.Just("9.3"),
		rapid.Just("39"),
		rapid.Just("40"),
		rapid.Just("rolling"),
		rapid.StringMatching(`[0-9]{1,2}(\.[0-9]{1,2})?`),
	).Draw(t, "version")

	// Generate optional extra fields that might appear in os-release
	prettyName := rapid.OneOf(
		rapid.Just(fmt.Sprintf("%s %s", distroInfo[id].Name, version)),
		rapid.Just(fmt.Sprintf("%s Linux %s", distroInfo[id].Name, version)),
		rapid.Just(""),
	).Draw(t, "pretty_name")

	// Build the os-release content with optional variations
	var lines []string

	// Optionally add comments and blank lines before content
	if rapid.Bool().Draw(t, "has_leading_comment") {
		lines = append(lines, "# This is a comment")
		lines = append(lines, "")
	}

	// ID field - may or may not be quoted
	if rapid.Bool().Draw(t, "id_quoted") {
		lines = append(lines, fmt.Sprintf(`ID="%s"`, id))
	} else {
		lines = append(lines, fmt.Sprintf("ID=%s", id))
	}

	// VERSION_ID field - may or may not be quoted
	if rapid.Bool().Draw(t, "version_quoted") {
		lines = append(lines, fmt.Sprintf(`VERSION_ID="%s"`, version))
	} else {
		lines = append(lines, fmt.Sprintf("VERSION_ID=%s", version))
	}

	// Optional PRETTY_NAME
	if prettyName != "" {
		lines = append(lines, fmt.Sprintf(`PRETTY_NAME="%s"`, prettyName))
	}

	// Optionally add other common fields
	if rapid.Bool().Draw(t, "has_name") {
		lines = append(lines, fmt.Sprintf(`NAME="%s"`, distroInfo[id].Name))
	}

	if rapid.Bool().Draw(t, "has_trailing_newline") {
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")
	return content, id, version
}

// TestOSDetection_SupportedDistros verifies that for any valid os-release content
// with a supported distro ID, DetectOS correctly extracts the distro name, version,
// and package manager.
func TestOSDetection_SupportedDistros(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		content, id, version := genOSReleaseContent(t)

		result, err := install.DetectOS(content)
		if err != nil {
			t.Fatalf("DetectOS returned error for supported distro %q: %v\ncontent:\n%s",
				id, err, content)
		}

		expected := distroInfo[id]

		// Property: distro name SHALL be correctly extracted
		if result.Distro != expected.Name {
			t.Fatalf("DetectOS distro mismatch: got %q, want %q for ID=%q\ncontent:\n%s",
				result.Distro, expected.Name, id, content)
		}

		// Property: version SHALL be correctly extracted
		if result.Version != version {
			t.Fatalf("DetectOS version mismatch: got %q, want %q for ID=%q\ncontent:\n%s",
				result.Version, version, id, content)
		}

		// Property: package manager SHALL be correctly mapped
		if result.PkgManager != expected.PkgManager {
			t.Fatalf("DetectOS pkg manager mismatch: got %q, want %q for ID=%q\ncontent:\n%s",
				result.PkgManager, expected.PkgManager, id, content)
		}
	})
}

// TestOSDetection_UnsupportedDistros verifies that for any os-release content
// with an unsupported distro ID, DetectOS returns ErrUnsupportedOS.
func TestOSDetection_UnsupportedDistros(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random ID that is NOT in the supported list
		unsupportedID := rapid.StringMatching(`[a-z]{3,12}`).Draw(t, "unsupported_id")

		// Skip if it accidentally matches a supported ID
		for _, supported := range supportedIDs {
			if unsupportedID == supported {
				t.Skip("generated ID matches supported distro")
			}
		}

		content := fmt.Sprintf("ID=%s\nVERSION_ID=1.0\n", unsupportedID)

		_, err := install.DetectOS(content)

		// Property: unsupported distro SHALL return ErrUnsupportedOS
		if err != install.ErrUnsupportedOS {
			t.Fatalf("DetectOS should return ErrUnsupportedOS for ID=%q, got: %v",
				unsupportedID, err)
		}
	})
}

// TestOSDetection_MissingID verifies that os-release content without an ID field
// returns ErrMissingID.
func TestOSDetection_MissingID(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate content without an ID field
		version := rapid.StringMatching(`[0-9]{1,2}\.[0-9]{1,2}`).Draw(t, "version")
		prettyName := rapid.StringMatching(`[A-Za-z ]{5,30}`).Draw(t, "pretty_name")

		content := fmt.Sprintf("VERSION_ID=%s\nPRETTY_NAME=\"%s\"\n", version, prettyName)

		_, err := install.DetectOS(content)

		// Property: missing ID SHALL return ErrMissingID
		if err != install.ErrMissingID {
			t.Fatalf("DetectOS should return ErrMissingID for content without ID, got: %v\ncontent:\n%s",
				err, content)
		}
	})
}

// --- Property 12: Install Script Idempotency ---

// TestInstallIdempotency_SameVersion verifies that when the installed binary version
// matches the target version, the install logic SHALL NOT download a new binary,
// SHALL preserve config, and SHALL ensure service is enabled+running.
func TestInstallIdempotency_SameVersion(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random version string
		version := rapid.StringMatching(`[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}`).Draw(t, "version")

		// System state: binary exists at target version, config exists, service running
		state := install.InstallState{
			BinaryExists:   true,
			BinaryVersion:  version,
			ConfigExists:   rapid.Bool().Draw(t, "config_exists"),
			ServiceEnabled: true,
			ServiceRunning: rapid.Bool().Draw(t, "service_running"),
		}

		decision := install.DecideInstallAction(state, version)

		// Property: same version SHALL NOT trigger download
		if decision.ShouldDownload {
			t.Fatalf("Idempotency violated: ShouldDownload=true when version matches (%s)",
				version)
		}

		// Property: existing config SHALL be preserved
		if state.ConfigExists && !decision.PreserveConfig {
			t.Fatalf("Idempotency violated: PreserveConfig=false when config exists")
		}

		// Property: service SHALL remain enabled
		if !decision.EnsureEnabled {
			t.Fatalf("Idempotency violated: EnsureEnabled=false")
		}

		// Property: service SHALL remain running
		if !decision.EnsureRunning {
			t.Fatalf("Idempotency violated: EnsureRunning=false")
		}

		// Property: should NOT stop service when version matches
		if decision.ShouldStopService {
			t.Fatalf("Idempotency violated: ShouldStopService=true when version matches")
		}
	})
}

// TestInstallIdempotency_DifferentVersion verifies that when the installed binary version
// differs from the target version, the install logic SHALL download the new binary and
// stop the running service before replacement.
func TestInstallIdempotency_DifferentVersion(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate two different version strings
		currentVersion := rapid.StringMatching(`[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}`).Draw(t, "current_version")
		targetVersion := rapid.StringMatching(`[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}`).Draw(t, "target_version")

		// Ensure versions are different
		if currentVersion == targetVersion {
			t.Skip("generated same version for current and target")
		}

		serviceRunning := rapid.Bool().Draw(t, "service_running")

		state := install.InstallState{
			BinaryExists:   true,
			BinaryVersion:  currentVersion,
			ConfigExists:   true,
			ServiceEnabled: true,
			ServiceRunning: serviceRunning,
		}

		decision := install.DecideInstallAction(state, targetVersion)

		// Property: different version SHALL trigger download
		if !decision.ShouldDownload {
			t.Fatalf("Update not triggered: ShouldDownload=false when versions differ (current=%s, target=%s)",
				currentVersion, targetVersion)
		}

		// Property: config SHALL still be preserved
		if !decision.PreserveConfig {
			t.Fatalf("Config not preserved during update")
		}

		// Property: service SHALL be stopped before replacement if running
		if serviceRunning && !decision.ShouldStopService {
			t.Fatalf("Service not stopped before binary replacement when running")
		}

		// Property: service SHALL be ensured enabled+running after update
		if !decision.EnsureEnabled || !decision.EnsureRunning {
			t.Fatalf("Service not ensured enabled+running after update")
		}
	})
}

// TestInstallIdempotency_FreshInstall verifies that when no binary exists,
// the install logic SHALL download the binary without trying to stop a service.
func TestInstallIdempotency_FreshInstall(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		targetVersion := rapid.StringMatching(`[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}`).Draw(t, "target_version")

		state := install.InstallState{
			BinaryExists:   false,
			BinaryVersion:  "",
			ConfigExists:   false,
			ServiceEnabled: false,
			ServiceRunning: false,
		}

		decision := install.DecideInstallAction(state, targetVersion)

		// Property: fresh install SHALL download binary
		if !decision.ShouldDownload {
			t.Fatalf("Fresh install did not trigger download")
		}

		// Property: fresh install SHALL NOT try to stop service
		if decision.ShouldStopService {
			t.Fatalf("Fresh install tried to stop non-existent service")
		}

		// Property: fresh install SHALL NOT preserve config (none exists)
		if decision.PreserveConfig {
			t.Fatalf("Fresh install tried to preserve non-existent config")
		}

		// Property: fresh install SHALL ensure service enabled+running
		if !decision.EnsureEnabled || !decision.EnsureRunning {
			t.Fatalf("Fresh install did not ensure service enabled+running")
		}
	})
}
