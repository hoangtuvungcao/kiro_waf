package diagnostics

import (
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
