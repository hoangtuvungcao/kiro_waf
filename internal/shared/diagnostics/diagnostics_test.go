package diagnostics

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kiro_waf/internal/shared/config"
)

func TestBuildPreflightPassesCoreChecksWithFixtures(t *testing.T) {
	dir := t.TempDir()
	osRelease := filepath.Join(dir, "os-release")
	if err := os.WriteFile(osRelease, []byte("ID=ubuntu\nVERSION_ID=\"22.04\"\nPRETTY_NAME=\"Ubuntu 22.04 LTS\"\n"), 0o644); err != nil {
		t.Fatalf("write os-release: %v", err)
	}
	report := BuildPreflight(testRuntime(dir), PreflightOptions{
		OSReleasePath: osRelease,
		EffectiveUID:  0,
		WritableRoot:  dir,
		CommandLookup: func(name string) (string, error) {
			return "/usr/bin/" + name, nil
		},
		Now: time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC),
	})
	if report.Status != "pass" {
		t.Fatalf("status = %s, checks = %#v", report.Status, report.Checks)
	}
}

func TestBuildPreflightWarnsMissingCommands(t *testing.T) {
	dir := t.TempDir()
	osRelease := filepath.Join(dir, "os-release")
	if err := os.WriteFile(osRelease, []byte("ID=ubuntu\nVERSION_ID=\"22.04\"\nPRETTY_NAME=\"Ubuntu 22.04 LTS\"\n"), 0o644); err != nil {
		t.Fatalf("write os-release: %v", err)
	}
	report := BuildPreflight(testRuntime(dir), PreflightOptions{
		OSReleasePath: osRelease,
		EffectiveUID:  0,
		WritableRoot:  dir,
		CommandLookup: func(name string) (string, error) {
			return "", errors.New("missing")
		},
	})
	if report.Status != "warn" {
		t.Fatalf("status = %s, want warn; checks=%#v", report.Status, report.Checks)
	}
}

func TestBuildPreflightChecksXDPToolchainWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	osRelease := filepath.Join(dir, "os-release")
	if err := os.WriteFile(osRelease, []byte("ID=ubuntu\nVERSION_ID=\"22.04\"\nPRETTY_NAME=\"Ubuntu 22.04 LTS\"\n"), 0o644); err != nil {
		t.Fatalf("write os-release: %v", err)
	}
	runtime := testRuntime(dir)
	runtime.XDP = config.RuntimeXDP{Enabled: true}
	seen := map[string]bool{}
	report := BuildPreflight(runtime, PreflightOptions{
		OSReleasePath: osRelease,
		EffectiveUID:  0,
		WritableRoot:  dir,
		CommandLookup: func(name string) (string, error) {
			seen[name] = true
			return "/usr/bin/" + name, nil
		},
	})
	if report.Status != "pass" {
		t.Fatalf("status = %s, checks = %#v", report.Status, report.Checks)
	}
	for _, name := range []string{"ip", "bpftool", "clang", "llvm-objdump"} {
		if !seen[name] {
			t.Fatalf("expected XDP tool lookup for %s, seen=%#v", name, seen)
		}
	}
}

func TestBuildHealthFailsWithoutAdminAllowlist(t *testing.T) {
	runtime := testRuntime(t.TempDir())
	runtime.AdminCIDRs = nil
	runtime.Firewall.AdminCIDRs = nil
	report := BuildHealth(runtime, PreflightReport{}, time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC))
	if report.Status != "fail" {
		t.Fatalf("status = %s, want fail; checks=%#v", report.Status, report.Checks)
	}
}

func TestSetModeFileUpdatesYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kiro.yaml")
	if err := os.WriteFile(path, []byte("mode: full\nplan: school_smb\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := SetModeFile(path, "server"); err != nil {
		t.Fatalf("set mode: %v", err)
	}
	mode, err := ShowMode(path)
	if err != nil {
		t.Fatalf("show mode: %v", err)
	}
	if mode != "server" {
		t.Fatalf("mode = %s, want server", mode)
	}
}

func testRuntime(dir string) config.RuntimeConfig {
	return config.RuntimeConfig{
		SourceKind: config.KindAdvanced,
		Mode:       "full",
		Plan:       "school_smb",
		Paths: config.RuntimePaths{
			StateDir:          filepath.Join(dir, "state"),
			LastGoodConfigDir: filepath.Join(dir, "state", "last-good-config"),
		},
		AdminCIDRs: []string{"203.0.113.10/32"},
		Firewall: config.RuntimeFirewall{
			Enabled:      true,
			SSHAdminOnly: true,
			AdminCIDRs:   []string{"203.0.113.10/32"},
		},
		Sites:        []config.RuntimeSite{{ID: "site"}},
		BackendPools: []config.RuntimeBackendPool{{ID: "pool", Upstreams: []config.RuntimeUpstream{{ID: "upstream", URL: "http://127.0.0.1:3000"}}}},
		Telemetry: config.RuntimeTelemetry{
			Privacy: config.RuntimePrivacy{
				HashClientIP:  true,
				RedactSecrets: true,
			},
		},
	}
}

// --- Tests for BuildStatus ---

func TestBuildStatus_ReturnsCorrectFields(t *testing.T) {
	dir := t.TempDir()
	runtime := testRuntime(dir)
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	status := BuildStatus(runtime, now)

	if status.Mode != "full" {
		t.Errorf("status.Mode = %q, want 'full'", status.Mode)
	}
	if status.Version == "" {
		t.Error("status.Version is empty")
	}
	if status.GeneratedAt != "2026-06-01T12:00:00Z" {
		t.Errorf("status.GeneratedAt = %q, want '2026-06-01T12:00:00Z'", status.GeneratedAt)
	}
	if status.Plan != "school_smb" {
		t.Errorf("status.Plan = %q, want 'school_smb'", status.Plan)
	}
	if status.Sites != 1 {
		t.Errorf("status.Sites = %d, want 1", status.Sites)
	}
}

func TestBuildStatus_ServerMode(t *testing.T) {
	dir := t.TempDir()
	runtime := testRuntime(dir)
	runtime.Mode = "server"
	runtime.Sites = nil
	status := BuildStatus(runtime, time.Now())

	if status.Mode != "server" {
		t.Errorf("status.Mode = %q, want 'server'", status.Mode)
	}
	if status.ProxyEnabled {
		t.Error("ProxyEnabled should be false in server mode")
	}
}

func TestBuildStatus_ZeroTime(t *testing.T) {
	dir := t.TempDir()
	runtime := testRuntime(dir)
	status := BuildStatus(runtime, time.Time{})
	if status.GeneratedAt == "" {
		t.Error("GeneratedAt should be set even with zero time input")
	}
}

// --- Tests for BuildSystemReport ---

func TestBuildSystemReport_ReturnsCorrectFields(t *testing.T) {
	dir := t.TempDir()
	runtime := testRuntime(dir)
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	report := BuildSystemReport(runtime, now)

	if report.Version == "" {
		t.Error("report.Version is empty")
	}
	if report.GoVersion == "" {
		t.Error("report.GoVersion is empty")
	}
	if report.OS == "" {
		t.Error("report.OS is empty")
	}
	if report.Arch == "" {
		t.Error("report.Arch is empty")
	}
	if report.NumCPU == 0 {
		t.Error("report.NumCPU is 0")
	}
	if report.Config.Mode != "full" {
		t.Errorf("report.Config.Mode = %q, want 'full'", report.Config.Mode)
	}
	if report.GeneratedAt != "2026-06-01T12:00:00Z" {
		t.Errorf("report.GeneratedAt = %q", report.GeneratedAt)
	}
}

func TestBuildSystemReport_ZeroTime(t *testing.T) {
	dir := t.TempDir()
	runtime := testRuntime(dir)
	report := BuildSystemReport(runtime, time.Time{})
	if report.GeneratedAt == "" {
		t.Error("GeneratedAt should be set even with zero time input")
	}
}

// --- Tests for MarshalJSON ---

func TestMarshalJSON_ProducesValidJSON(t *testing.T) {
	dir := t.TempDir()
	runtime := testRuntime(dir)
	status := BuildStatus(runtime, time.Now())

	data, err := MarshalJSON(status)
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("MarshalJSON returned empty data")
	}
	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("MarshalJSON output is not valid JSON: %v", err)
	}
}

// --- Tests for mode checks ---

func TestModeCheck_ValidModes(t *testing.T) {
	for _, mode := range []string{"server", "full"} {
		runtime := config.RuntimeConfig{Mode: mode}
		check := modeCheck(runtime)
		if check.Status != "pass" {
			t.Errorf("modeCheck(%q) status = %q, want 'pass'", mode, check.Status)
		}
	}
}

func TestModeCheck_InvalidMode(t *testing.T) {
	runtime := config.RuntimeConfig{Mode: "invalid"}
	check := modeCheck(runtime)
	if check.Status != "fail" {
		t.Errorf("modeCheck('invalid') status = %q, want 'fail'", check.Status)
	}
}

// --- Tests for firewall safety check ---

func TestFirewallSafetyCheck_Disabled(t *testing.T) {
	runtime := config.RuntimeConfig{Firewall: config.RuntimeFirewall{Enabled: false}}
	check := firewallSafetyCheck(runtime)
	if check.Status != "warn" {
		t.Errorf("firewallSafetyCheck(disabled) status = %q, want 'warn'", check.Status)
	}
}

func TestFirewallSafetyCheck_SSHAdminOnlyWithoutCIDRs(t *testing.T) {
	runtime := config.RuntimeConfig{
		Firewall: config.RuntimeFirewall{
			Enabled:      true,
			SSHAdminOnly: true,
			AdminCIDRs:   nil,
		},
	}
	check := firewallSafetyCheck(runtime)
	if check.Status != "fail" {
		t.Errorf("firewallSafetyCheck(ssh_admin_only without CIDRs) status = %q, want 'fail'", check.Status)
	}
}

// --- Tests for full mode check ---

func TestFullModeCheck_ServerMode(t *testing.T) {
	runtime := config.RuntimeConfig{Mode: "server"}
	check := fullModeCheck(runtime)
	if check.Status != "pass" {
		t.Errorf("fullModeCheck(server) status = %q, want 'pass'", check.Status)
	}
}

func TestFullModeCheck_FullModeWithoutSites(t *testing.T) {
	runtime := config.RuntimeConfig{Mode: "full", Sites: nil}
	check := fullModeCheck(runtime)
	if check.Status != "fail" {
		t.Errorf("fullModeCheck(full without sites) status = %q, want 'fail'", check.Status)
	}
}

// --- Tests for privacy check ---

func TestPrivacyCheck_SendsRequestBody(t *testing.T) {
	runtime := config.RuntimeConfig{
		Telemetry: config.RuntimeTelemetry{
			Privacy: config.RuntimePrivacy{SendRequestBody: true, RedactSecrets: true},
		},
	}
	check := privacyCheck(runtime)
	if check.Status != "fail" {
		t.Errorf("privacyCheck(SendRequestBody) status = %q, want 'fail'", check.Status)
	}
}

func TestPrivacyCheck_RedactDisabled(t *testing.T) {
	runtime := config.RuntimeConfig{
		Telemetry: config.RuntimeTelemetry{
			Privacy: config.RuntimePrivacy{RedactSecrets: false},
		},
	}
	check := privacyCheck(runtime)
	if check.Status != "warn" {
		t.Errorf("privacyCheck(RedactSecrets=false) status = %q, want 'warn'", check.Status)
	}
}

// --- Tests for ShowMode ---

func TestShowMode_ReturnsMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kiro.yaml")
	if err := os.WriteFile(path, []byte("mode: full\nplan: test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mode, err := ShowMode(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode != "full" {
		t.Errorf("ShowMode() = %q, want 'full'", mode)
	}
}

func TestShowMode_ErrorOnMissingMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kiro.yaml")
	if err := os.WriteFile(path, []byte("plan: test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ShowMode(path)
	if err == nil {
		t.Error("ShowMode should return error when mode is not set")
	}
}

func TestShowMode_ErrorOnMissingFile(t *testing.T) {
	_, err := ShowMode("/nonexistent/path.yaml")
	if err == nil {
		t.Error("ShowMode should return error for missing file")
	}
}
