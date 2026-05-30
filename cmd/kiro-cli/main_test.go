package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"kiro_waf/internal/shared/buildinfo"
	"kiro_waf/internal/shared/config"
	"kiro_waf/internal/shared/diagnostics"
	"kiro_waf/internal/shared/installer"
	"kiro_waf/internal/shared/machinefingerprint"
)

// buildCLI builds the kiro-cli binary for testing and returns the path.
func buildCLI(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	binPath := filepath.Join(dir, "kiro-cli")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = filepath.Join(projectRoot(t), "cmd", "kiro-cli")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build kiro-cli: %v\n%s", err, out)
	}
	return binPath
}

// projectRoot returns the project root directory.
func projectRoot(t *testing.T) string {
	t.Helper()
	// Walk up from cmd/kiro-cli to find go.mod
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root")
		}
		dir = parent
	}
}

// writeTestConfig writes a minimal valid config YAML for testing.
func writeTestConfig(t *testing.T, dir string) string {
	t.Helper()
	configPath := filepath.Join(dir, "kiro-test.yaml")
	content := `mode: server
plan: community
license_key: KIRO-TEST-0000-0000
admin:
  allow_ips:
    - 203.0.113.10/32
server:
  interface: eth0
  ssh_port: 22
protection:
  profile: balanced
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return configPath
}

// runCLI executes the CLI binary with the given args and returns stdout, stderr, and exit code.
func runCLI(t *testing.T, binPath string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("run CLI: %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

// --- Test: version command ---

func TestVersionCommand_ReturnsSemver(t *testing.T) {
	bin := buildCLI(t)
	stdout, _, exitCode := runCLI(t, bin, "version")
	if exitCode != 0 {
		t.Fatalf("version command exited with code %d, want 0", exitCode)
	}
	version := strings.TrimSpace(stdout)
	// Semver pattern: X.Y.Z or X.Y.Z-suffix
	semverPattern := regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-zA-Z0-9._-]+)?$`)
	if !semverPattern.MatchString(version) {
		t.Errorf("version output %q does not match semver pattern X.Y.Z", version)
	}
}

// --- Test: license fingerprint command ---

func TestLicenseFingerprintCommand_ReturnsValidHex(t *testing.T) {
	bin := buildCLI(t)
	stdout, _, exitCode := runCLI(t, bin, "license", "fingerprint")
	if exitCode != 0 {
		t.Skipf("fingerprint command exited with code %d (may need /etc/machine-id)", exitCode)
	}
	output := strings.TrimSpace(stdout)
	// Output format is "sha256:" + 64 hex chars
	hexPattern := regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	if !hexPattern.MatchString(output) {
		t.Errorf("fingerprint output %q does not match expected pattern sha256:<64 hex chars>", output)
	}
}

func TestLicenseFingerprintCommand_AcceptsSalt(t *testing.T) {
	bin := buildCLI(t)
	stdout1, _, exit1 := runCLI(t, bin, "license", "fingerprint", "--salt", "test-salt-1")
	stdout2, _, exit2 := runCLI(t, bin, "license", "fingerprint", "--salt", "test-salt-2")
	if exit1 != 0 || exit2 != 0 {
		t.Skipf("fingerprint command failed (may need /etc/machine-id)")
	}
	// Different salts should produce different fingerprints
	fp1 := strings.TrimSpace(stdout1)
	fp2 := strings.TrimSpace(stdout2)
	if fp1 == fp2 {
		t.Errorf("different salts produced same fingerprint: %s", fp1)
	}
}

func TestLicenseFingerprintCommand_Deterministic(t *testing.T) {
	bin := buildCLI(t)
	stdout1, _, exit1 := runCLI(t, bin, "license", "fingerprint", "--salt", "same-salt")
	stdout2, _, exit2 := runCLI(t, bin, "license", "fingerprint", "--salt", "same-salt")
	if exit1 != 0 || exit2 != 0 {
		t.Skipf("fingerprint command failed (may need /etc/machine-id)")
	}
	fp1 := strings.TrimSpace(stdout1)
	fp2 := strings.TrimSpace(stdout2)
	if fp1 != fp2 {
		t.Errorf("same salt produced different fingerprints: %s vs %s", fp1, fp2)
	}
}

// --- Test: status command ---

func TestStatusCommand_ReturnsJSON(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir)

	stdout, _, exitCode := runCLI(t, bin, "status", "--config", configPath)
	if exitCode != 0 {
		t.Fatalf("status command exited with code %d", exitCode)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("status output is not valid JSON: %v\nOutput: %s", err, stdout)
	}

	// Check required fields: mode, version
	requiredFields := []string{"mode", "version"}
	for _, field := range requiredFields {
		if _, ok := result[field]; !ok {
			t.Errorf("status JSON missing required field %q", field)
		}
	}

	// Verify mode is "server" as configured
	if mode, ok := result["mode"].(string); !ok || mode != "server" {
		t.Errorf("status mode = %v, want 'server'", result["mode"])
	}
}

// --- Test: health command ---

func TestHealthCommand_ReturnsJSON(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir)
	osRelease := filepath.Join(dir, "os-release")
	if err := os.WriteFile(osRelease, []byte("ID=ubuntu\nVERSION_ID=\"22.04\"\nPRETTY_NAME=\"Ubuntu 22.04 LTS\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, exitCode := runCLI(t, bin, "health",
		"--config", configPath,
		"--os-release", osRelease,
		"--preflight-writable-root", dir,
		"--skip-command-checks",
	)
	if exitCode != 0 {
		t.Fatalf("health command exited with code %d", exitCode)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("health output is not valid JSON: %v\nOutput: %s", err, stdout)
	}

	// Check overall status field exists and is one of expected values
	status, ok := result["status"].(string)
	if !ok {
		t.Fatal("health JSON missing 'status' field")
	}
	validStatuses := map[string]bool{"pass": true, "warn": true, "fail": true}
	if !validStatuses[status] {
		t.Errorf("health status = %q, want one of pass/warn/fail", status)
	}

	// Check checks array exists
	if _, ok := result["checks"]; !ok {
		t.Error("health JSON missing 'checks' field")
	}
}

// --- Test: preflight command ---

func TestPreflightCommand_ReturnsJSON(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir)
	osRelease := filepath.Join(dir, "os-release")
	if err := os.WriteFile(osRelease, []byte("ID=ubuntu\nVERSION_ID=\"22.04\"\nPRETTY_NAME=\"Ubuntu 22.04 LTS\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, exitCode := runCLI(t, bin, "preflight",
		"--config", configPath,
		"--os-release", osRelease,
		"--preflight-writable-root", dir,
		"--skip-command-checks",
	)
	if exitCode != 0 {
		t.Fatalf("preflight command exited with code %d", exitCode)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("preflight output is not valid JSON: %v\nOutput: %s", err, stdout)
	}

	// Check required fields
	if _, ok := result["status"]; !ok {
		t.Error("preflight JSON missing 'status' field")
	}
	if _, ok := result["checks"]; !ok {
		t.Error("preflight JSON missing 'checks' field")
	}

	// Verify checks contain OS and root checks
	checks, ok := result["checks"].([]interface{})
	if !ok {
		t.Fatal("preflight 'checks' is not an array")
	}
	checkNames := make(map[string]bool)
	for _, c := range checks {
		if check, ok := c.(map[string]interface{}); ok {
			if name, ok := check["name"].(string); ok {
				checkNames[name] = true
			}
		}
	}
	expectedChecks := []string{"os", "ubuntu_release", "root"}
	for _, name := range expectedChecks {
		if !checkNames[name] {
			t.Errorf("preflight missing check %q, got checks: %v", name, checkNames)
		}
	}
}

// --- Test: mode set command ---

func TestModeSetCommand_RejectsInvalidMode(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "kiro.yaml")
	if err := os.WriteFile(configPath, []byte("mode: server\nplan: community\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	invalidModes := []string{"invalid", "hybrid", "test", "", "SERVER", "FULL"}
	for _, mode := range invalidModes {
		t.Run("mode="+mode, func(t *testing.T) {
			_, _, exitCode := runCLI(t, bin, "mode", "set", "--config", configPath, "--mode", mode)
			if exitCode != 1 {
				t.Errorf("mode set %q exited with code %d, want 1", mode, exitCode)
			}
		})
	}
}

func TestModeSetCommand_AcceptsValidModes(t *testing.T) {
	bin := buildCLI(t)

	for _, mode := range []string{"server", "full"} {
		t.Run("mode="+mode, func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "kiro.yaml")
			if err := os.WriteFile(configPath, []byte("mode: server\nplan: community\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			_, _, exitCode := runCLI(t, bin, "mode", "set", "--config", configPath, "--mode", mode)
			if exitCode != 0 {
				t.Errorf("mode set %q exited with code %d, want 0", mode, exitCode)
			}
		})
	}
}

// --- Test: install apply-lab access control ---

func TestInstallApplyLabCommand_RejectsWrongAck(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir)
	osRelease := filepath.Join(dir, "os-release")
	if err := os.WriteFile(osRelease, []byte("ID=ubuntu\nVERSION_ID=\"22.04\"\nPRETTY_NAME=\"Ubuntu 22.04 LTS\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, stderr, exitCode := runCLI(t, bin, "install", "apply-lab",
		"--config", configPath,
		"--install-root", dir,
		"--ack", "WRONG_VALUE",
		"--os-release", osRelease,
		"--skip-os-check",
	)
	if exitCode != 1 {
		t.Errorf("install apply-lab with wrong ack exited with code %d, want 1\nstderr: %s", exitCode, stderr)
	}
	if !strings.Contains(stderr, "ack") && !strings.Contains(stderr, "KIRO_LAB_INSTALL_APPLY") {
		t.Errorf("error message should mention ack requirement, got: %s", stderr)
	}
}

func TestInstallApplyLabCommand_RejectsNonRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("test requires non-root user")
	}
	bin := buildCLI(t)
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir)
	osRelease := filepath.Join(dir, "os-release")
	if err := os.WriteFile(osRelease, []byte("ID=ubuntu\nVERSION_ID=\"22.04\"\nPRETTY_NAME=\"Ubuntu 22.04 LTS\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// With correct ack but no --install-root (real root apply), non-root should be rejected
	_, stderr, exitCode := runCLI(t, bin, "install", "apply-lab",
		"--config", configPath,
		"--ack", "KIRO_LAB_INSTALL_APPLY",
		"--os-release", osRelease,
		"--skip-os-check",
	)
	if exitCode != 1 {
		t.Errorf("install apply-lab without root exited with code %d, want 1\nstderr: %s", exitCode, stderr)
	}
	if !strings.Contains(stderr, "root") {
		t.Errorf("error message should mention root requirement, got: %s", stderr)
	}
}

// --- Test: invalid command ---

func TestInvalidCommand_ExitsCode2(t *testing.T) {
	bin := buildCLI(t)
	_, _, exitCode := runCLI(t, bin, "nonexistent-command")
	if exitCode != 2 {
		t.Errorf("invalid command exited with code %d, want 2", exitCode)
	}
}

func TestNoArgs_ExitsCode2(t *testing.T) {
	bin := buildCLI(t)
	_, _, exitCode := runCLI(t, bin)
	if exitCode != 2 {
		t.Errorf("no args exited with code %d, want 2", exitCode)
	}
}

// --- Test: report command ---

func TestReportCommand_ReturnsJSON(t *testing.T) {
	bin := buildCLI(t)
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir)

	stdout, _, exitCode := runCLI(t, bin, "report", "--config", configPath)
	if exitCode != 0 {
		t.Fatalf("report command exited with code %d", exitCode)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("report output is not valid JSON: %v\nOutput: %s", err, stdout)
	}

	// Check required fields
	requiredFields := []string{"version", "generated_at", "go_version", "os", "arch", "config"}
	for _, field := range requiredFields {
		if _, ok := result[field]; !ok {
			t.Errorf("report JSON missing required field %q", field)
		}
	}
}

// ============================================================
// Unit tests that directly test internal functions for coverage
// ============================================================

// --- Unit test: version ---

func TestBuildInfoVersion_IsSemver(t *testing.T) {
	version := buildinfo.Version
	semverPattern := regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-zA-Z0-9._-]+)?$`)
	if !semverPattern.MatchString(version) {
		t.Errorf("buildinfo.Version %q does not match semver pattern", version)
	}
}

// --- Unit test: fingerprint ---

func TestMachineFingerprint_ProducesValidHex(t *testing.T) {
	// Create mock machine-id file
	dir := t.TempDir()
	machineIDPath := filepath.Join(dir, "machine-id")
	if err := os.WriteFile(machineIDPath, []byte("test-machine-id-12345\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	snapshot, err := machinefingerprint.Collect(machinefingerprint.Options{
		MachineIDPath: machineIDPath,
		Hostname:      "test-host",
	})
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	hash := snapshot.FingerprintHash("")
	// Should be "sha256:" + 64 hex chars
	hexPattern := regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	if !hexPattern.MatchString(hash) {
		t.Errorf("FingerprintHash() = %q, want sha256:<64 hex chars>", hash)
	}
}

func TestMachineFingerprint_SaltChangesOutput(t *testing.T) {
	dir := t.TempDir()
	machineIDPath := filepath.Join(dir, "machine-id")
	if err := os.WriteFile(machineIDPath, []byte("test-machine-id-12345\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	snapshot, err := machinefingerprint.Collect(machinefingerprint.Options{
		MachineIDPath: machineIDPath,
		Hostname:      "test-host",
	})
	if err != nil {
		t.Fatal(err)
	}

	hash1 := snapshot.FingerprintHash("salt-a")
	hash2 := snapshot.FingerprintHash("salt-b")
	if hash1 == hash2 {
		t.Errorf("different salts produced same hash: %s", hash1)
	}
}

func TestMachineFingerprint_Deterministic(t *testing.T) {
	dir := t.TempDir()
	machineIDPath := filepath.Join(dir, "machine-id")
	if err := os.WriteFile(machineIDPath, []byte("test-machine-id-12345\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	snapshot, err := machinefingerprint.Collect(machinefingerprint.Options{
		MachineIDPath: machineIDPath,
		Hostname:      "test-host",
	})
	if err != nil {
		t.Fatal(err)
	}

	hash1 := snapshot.FingerprintHash("same-salt")
	hash2 := snapshot.FingerprintHash("same-salt")
	if hash1 != hash2 {
		t.Errorf("same salt produced different hashes: %s vs %s", hash1, hash2)
	}
}

// --- Unit test: status ---

func TestBuildStatus_ContainsRequiredFields(t *testing.T) {
	runtime := testRuntimeConfig(t)
	status := diagnostics.BuildStatus(runtime, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	if status.Mode != "server" {
		t.Errorf("status.Mode = %q, want 'server'", status.Mode)
	}
	if status.Version == "" {
		t.Error("status.Version is empty")
	}
	if status.GeneratedAt == "" {
		t.Error("status.GeneratedAt is empty")
	}
}

// --- Unit test: health ---

func TestBuildHealth_ReturnsValidStatus(t *testing.T) {
	runtime := testRuntimeConfig(t)
	preflight := diagnostics.PreflightReport{
		Status: "pass",
		Checks: []diagnostics.Check{
			{Name: "os", Status: "pass", Message: "running on linux"},
		},
	}
	health := diagnostics.BuildHealth(runtime, preflight, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	validStatuses := map[string]bool{"pass": true, "warn": true, "fail": true}
	if !validStatuses[health.Status] {
		t.Errorf("health.Status = %q, want one of pass/warn/fail", health.Status)
	}
	if len(health.Checks) == 0 {
		t.Error("health.Checks is empty")
	}
}

// --- Unit test: preflight ---

func TestBuildPreflight_ContainsOSAndRootChecks(t *testing.T) {
	dir := t.TempDir()
	osRelease := filepath.Join(dir, "os-release")
	if err := os.WriteFile(osRelease, []byte("ID=ubuntu\nVERSION_ID=\"22.04\"\nPRETTY_NAME=\"Ubuntu 22.04 LTS\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	runtime := testRuntimeConfig(t)
	report := diagnostics.BuildPreflight(runtime, diagnostics.PreflightOptions{
		OSReleasePath:     osRelease,
		EffectiveUID:      0,
		WritableRoot:      dir,
		SkipCommandChecks: true,
		Now:               time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})

	checkNames := make(map[string]bool)
	for _, c := range report.Checks {
		checkNames[c.Name] = true
	}

	expectedChecks := []string{"os", "ubuntu_release", "root"}
	for _, name := range expectedChecks {
		if !checkNames[name] {
			t.Errorf("preflight missing check %q", name)
		}
	}
}

// --- Unit test: mode set validation ---

func TestSetModeFile_RejectsInvalidMode(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "kiro.yaml")
	if err := os.WriteFile(configPath, []byte("mode: server\nplan: community\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	invalidModes := []string{"invalid", "hybrid", "", "SERVER", "FULL"}
	for _, mode := range invalidModes {
		err := diagnostics.SetModeFile(configPath, mode)
		if err == nil {
			t.Errorf("SetModeFile(%q) should return error", mode)
		}
	}
}

func TestSetModeFile_AcceptsValidModes(t *testing.T) {
	for _, mode := range []string{"server", "full"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "kiro.yaml")
			if err := os.WriteFile(configPath, []byte("mode: server\nplan: community\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := diagnostics.SetModeFile(configPath, mode); err != nil {
				t.Errorf("SetModeFile(%q) returned error: %v", mode, err)
			}
			got, err := diagnostics.ShowMode(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if got != mode {
				t.Errorf("ShowMode() = %q, want %q", got, mode)
			}
		})
	}
}

// --- Unit test: install apply-lab access control ---

func TestApplyLabInstall_RejectsWrongAck(t *testing.T) {
	dir := t.TempDir()
	runtime := testRuntimeConfig(t)

	_, err := installer.ApplyLabInstall(runtime, installer.Options{
		ConfigPath:  filepath.Join(dir, "kiro.yaml"),
		InstallRoot: dir,
	}, installer.ApplyOptions{
		Ack:          "WRONG_VALUE",
		EffectiveUID: 0,
		SkipOSCheck:  true,
	})
	if err == nil {
		t.Error("ApplyLabInstall with wrong ack should return error")
	}
	if !strings.Contains(err.Error(), "ack") {
		t.Errorf("error should mention ack, got: %v", err)
	}
}

func TestApplyLabInstall_RejectsNonRootForRealRoot(t *testing.T) {
	dir := t.TempDir()
	runtime := testRuntimeConfig(t)

	// No install-root means real root apply, which requires UID 0
	_, err := installer.ApplyLabInstall(runtime, installer.Options{
		ConfigPath:  filepath.Join(dir, "kiro.yaml"),
		InstallRoot: "", // empty = real root
	}, installer.ApplyOptions{
		Ack:          "KIRO_LAB_INSTALL_APPLY",
		EffectiveUID: 1000, // non-root
		SkipOSCheck:  true,
	})
	if err == nil {
		t.Error("ApplyLabInstall without root should return error")
	}
	if !strings.Contains(err.Error(), "root") {
		t.Errorf("error should mention root, got: %v", err)
	}
}

// --- Unit test: report ---

func TestBuildSystemReport_ContainsRequiredFields(t *testing.T) {
	runtime := testRuntimeConfig(t)
	report := diagnostics.BuildSystemReport(runtime, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	if report.Version == "" {
		t.Error("report.Version is empty")
	}
	if report.GeneratedAt == "" {
		t.Error("report.GeneratedAt is empty")
	}
	if report.GoVersion == "" {
		t.Error("report.GoVersion is empty")
	}
	if report.OS == "" {
		t.Error("report.OS is empty")
	}
	if report.Config.Mode != "server" {
		t.Errorf("report.Config.Mode = %q, want 'server'", report.Config.Mode)
	}
}

// --- Unit test: usage (invalid command) ---

func TestUsage_ExitsCode2_ViaExec(t *testing.T) {
	bin := buildCLI(t)
	_, _, exitCode := runCLI(t, bin, "nonexistent")
	if exitCode != 2 {
		t.Errorf("invalid command exit code = %d, want 2", exitCode)
	}
}

// --- Unit test: writeJSON helper ---

func TestWriteJSON_ProducesValidJSON(t *testing.T) {
	runtime := testRuntimeConfig(t)
	status := diagnostics.BuildStatus(runtime, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
}

// --- Unit test: mustLoadRuntime with invalid path ---

func TestMustLoadRuntime_InvalidPath(t *testing.T) {
	bin := buildCLI(t)
	_, stderr, exitCode := runCLI(t, bin, "status", "--config", "/nonexistent/path.yaml")
	if exitCode != 1 {
		t.Errorf("status with invalid config exited with code %d, want 1", exitCode)
	}
	if !strings.Contains(stderr, "runtime config load failed") {
		t.Errorf("expected config load error, got: %s", stderr)
	}
}

// --- Helper: test runtime config ---

func testRuntimeConfig(t *testing.T) config.RuntimeConfig {
	t.Helper()
	return config.RuntimeConfig{
		SourceKind: config.KindTenant,
		Mode:       "server",
		Plan:       "community",
		Paths: config.RuntimePaths{
			StateDir:          filepath.Join(t.TempDir(), "state"),
			LastGoodConfigDir: filepath.Join(t.TempDir(), "state", "last-good-config"),
		},
		AdminCIDRs: []string{"203.0.113.10/32"},
		Firewall: config.RuntimeFirewall{
			Enabled:      true,
			SSHAdminOnly: true,
			AdminCIDRs:   []string{"203.0.113.10/32"},
		},
		Telemetry: config.RuntimeTelemetry{
			Privacy: config.RuntimePrivacy{
				RedactSecrets: true,
			},
		},
	}
}
