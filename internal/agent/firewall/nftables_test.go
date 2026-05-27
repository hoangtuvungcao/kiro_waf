package firewall

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kiro_waf/internal/shared/config"
	"kiro_waf/internal/shared/storage"
)

func TestGenerateRejectsMissingAdminCIDRWhenSSHAdminOnly(t *testing.T) {
	runtime := serverRuntime()
	runtime.Firewall.AdminCIDRs = nil
	runtime.AdminCIDRs = nil
	if _, err := GenerateNftables(runtime, Options{}); err == nil {
		t.Fatal("expected missing admin CIDR rejection")
	}
}

func TestGenerateServerModeIsDeterministic(t *testing.T) {
	runtime := serverRuntime()
	first, err := GenerateNftables(runtime, Options{})
	if err != nil {
		t.Fatalf("generate first ruleset: %v", err)
	}
	second, err := GenerateNftables(runtime, Options{})
	if err != nil {
		t.Fatalf("generate second ruleset: %v", err)
	}
	if first.Ruleset != second.Ruleset || first.SHA256 != second.SHA256 {
		t.Fatal("expected deterministic ruleset output")
	}
	assertContains(t, first.Ruleset, "set kiro_admin_v4")
	assertContains(t, first.Ruleset, "tcp dport 22 accept comment \"kiro_admin_ssh_v4\"")
	assertContains(t, first.Ruleset, "set kiro_temp_ban_v4")
	if strings.Contains(first.Ruleset, "kiro_cloudflare_direct_origin_drop") {
		t.Fatal("server mode should not include cloudflare direct origin drop")
	}
}

func TestGenerateFullModeWithCloudflareOriginLock(t *testing.T) {
	runtime := fullRuntime()
	plan, err := GenerateNftables(runtime, Options{
		CloudflareIPv4: []string{"203.0.113.0/24", "198.51.100.0/24"},
		CloudflareIPv6: []string{"2001:db8::/32"},
	})
	if err != nil {
		t.Fatalf("generate full ruleset: %v", err)
	}
	assertContains(t, plan.Ruleset, "set kiro_cloudflare_v4")
	assertContains(t, plan.Ruleset, "198.51.100.0/24")
	assertContains(t, plan.Ruleset, "203.0.113.0/24")
	assertContains(t, plan.Ruleset, "set kiro_cloudflare_v6")
	assertContains(t, plan.Ruleset, "2001:db8::/32")
	assertContains(t, plan.Ruleset, "kiro_cloudflare_origin_v4")
	assertContains(t, plan.Ruleset, "kiro_cloudflare_origin_v6")
	assertContains(t, plan.Ruleset, "kiro_cloudflare_direct_origin_drop")
}

func TestWriteLastGoodSnapshot(t *testing.T) {
	runtime := serverRuntime()
	plan, err := GenerateNftables(runtime, Options{})
	if err != nil {
		t.Fatalf("generate ruleset: %v", err)
	}
	path, err := WriteLastGoodSnapshot(t.TempDir(), runtime, plan, time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
	if filepath.Base(path) != "last-good-nftables.json" {
		t.Fatalf("snapshot path = %s", path)
	}
	var snapshot Snapshot
	if err := storage.ReadJSON(path, &snapshot); err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if snapshot.SHA256 != plan.SHA256 || snapshot.Ruleset != plan.Ruleset {
		t.Fatal("snapshot should contain generated ruleset and hash")
	}
}

func serverRuntime() config.RuntimeConfig {
	return config.RuntimeConfig{
		Mode:    "server",
		SSHPort: 22,
		Firewall: config.RuntimeFirewall{
			Enabled:               true,
			ProtectConntrack:      true,
			AllowPorts:            []int{22, 80},
			SSHAdminOnly:          true,
			AdminCIDRs:            []string{"203.0.113.10/32"},
			TemporaryBlockSeconds: 900,
		},
	}
}

func fullRuntime() config.RuntimeConfig {
	runtime := serverRuntime()
	runtime.Mode = "full"
	runtime.Firewall.AllowPorts = []int{22, 80, 443}
	runtime.CFOriginLock = config.RuntimeCloudflareOriginLock{
		Enabled:               true,
		RequireProxiedTraffic: true,
		BlockDirectOriginHTTP: true,
	}
	return runtime
}

func assertContains(t *testing.T, value string, want string) {
	t.Helper()
	if !strings.Contains(value, want) {
		t.Fatalf("expected ruleset to contain %q\n%s", want, value)
	}
}
