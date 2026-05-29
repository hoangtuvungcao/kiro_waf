package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckTenantConfig(t *testing.T) {
	res, err := CheckFile("../../../configs/kiro.example.yaml")
	if err != nil {
		t.Fatalf("check tenant config: %v", err)
	}
	if res.Kind != KindTenant {
		t.Fatalf("kind = %s, want %s", res.Kind, KindTenant)
	}
}

func TestCheckAdvancedConfig(t *testing.T) {
	res, err := CheckFile("../../../configs/kiro.advanced.example.yaml")
	if err != nil {
		t.Fatalf("check advanced config: %v", err)
	}
	if res.Kind != KindAdvanced {
		t.Fatalf("kind = %s, want %s", res.Kind, KindAdvanced)
	}
}

func TestCheckProviderConfig(t *testing.T) {
	res, err := CheckFile("../../../configs/provider.example.yaml")
	if err != nil {
		t.Fatalf("check provider config: %v", err)
	}
	if res.Kind != KindProvider {
		t.Fatalf("kind = %s, want %s", res.Kind, KindProvider)
	}
}

func TestLoadProviderConfigIncludesPlans(t *testing.T) {
	cfg, err := LoadProviderFile("../../../configs/provider.example.yaml")
	if err != nil {
		t.Fatalf("load provider config: %v", err)
	}
	plan := cfg.Licenses.Plans["school_smb"]
	if len(plan.AllowedModes) == 0 || len(plan.Features) == 0 {
		t.Fatal("expected provider license plans")
	}
}

func TestTenantRequiresWebsiteInFullMode(t *testing.T) {
	cfg := TenantConfig{
		Mode:       "full",
		Plan:       "school_smb",
		LicenseKey: "KIRO-TEST",
		Admin:      TenantAdmin{AllowIPs: []string{"203.0.113.10/32"}},
		Server:     TenantServer{Interface: "eth0", SSHPort: 22},
		Protection: TenantProtection{Profile: "balanced"},
		Website:    TenantWebsite{Enabled: true, TLSMode: "flexible_http"},
	}
	if err := ValidateTenant(cfg); err == nil {
		t.Fatal("expected missing website sites to fail")
	}
}

func TestLoadRuntimeExpandsTenantConfig(t *testing.T) {
	cfg, err := LoadRuntimeFile("../../../configs/kiro.example.yaml")
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	if cfg.SourceKind != KindTenant {
		t.Fatalf("source kind = %s, want tenant", cfg.SourceKind)
	}
	if len(cfg.Sites) != 1 {
		t.Fatalf("sites = %d, want 1", len(cfg.Sites))
	}
	if len(cfg.BackendPools) != 2 {
		t.Fatalf("backend pools = %d, want 2", len(cfg.BackendPools))
	}
	if cfg.Sites[0].Routes[0].BackendPool == cfg.Sites[0].DefaultBackendPool {
		t.Fatal("expected /api route to use a separate backend pool")
	}
}

func TestLoadRuntimeExpandsAdvancedConfig(t *testing.T) {
	cfg, err := LoadRuntimeFile("../../../configs/kiro.advanced.example.yaml")
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	if cfg.SourceKind != KindAdvanced {
		t.Fatalf("source kind = %s, want advanced", cfg.SourceKind)
	}
	if len(cfg.Sites) == 0 || len(cfg.BackendPools) == 0 {
		t.Fatal("expected sites and backend pools")
	}
	if cfg.License.File != "/etc/kiro/license.json" {
		t.Fatalf("license file = %q", cfg.License.File)
	}
	if cfg.Identity.FingerprintSaltID != "default-provider-key-2026" {
		t.Fatalf("fingerprint salt id = %q", cfg.Identity.FingerprintSaltID)
	}
	if !cfg.Firewall.Enabled || !cfg.Firewall.SSHAdminOnly {
		t.Fatal("expected advanced runtime firewall settings")
	}
	if !cfg.XDP.Enabled || cfg.XDP.Mode != "native" || cfg.XDP.ProgramPath != "/usr/lib/kiro/xdp/kiro_xdp_drop.o" || cfg.XDP.Section != "xdp" {
		t.Fatalf("unexpected runtime XDP config: %#v", cfg.XDP)
	}
	if !cfg.CFOriginLock.Enabled || cfg.CFOriginLock.IPv4File == "" || cfg.CFOriginLock.IPv6File == "" {
		t.Fatal("expected advanced runtime cloudflare origin lock settings")
	}
	if cfg.Sites[0].TLSMode != "flexible_http" {
		t.Fatalf("site tls mode = %q", cfg.Sites[0].TLSMode)
	}
	if cfg.Paths.StateDir != "/var/lib/kiro" || cfg.Paths.LastGoodConfigDir == "" {
		t.Fatalf("unexpected runtime paths: %#v", cfg.Paths)
	}
	if cfg.Safety.RollbackTimerSeconds != 60 {
		t.Fatalf("rollback timer = %d, want 60", cfg.Safety.RollbackTimerSeconds)
	}
	if !cfg.WAF.Enabled || cfg.WAF.Engine != "coraza" || !cfg.WAF.OWASPCRS || cfg.WAF.AnomalyThreshold != 5 {
		t.Fatalf("unexpected runtime WAF config: %#v", cfg.WAF)
	}
	if !cfg.Bot.Enabled || !cfg.Bot.CookieChallenge || cfg.Bot.ScoreChallenge != 50 || cfg.Bot.ScoreBlock != 80 {
		t.Fatalf("unexpected runtime bot config: %#v", cfg.Bot)
	}
	if !cfg.ResourceGovernor.Enabled {
		t.Fatal("expected advanced resource governor to be enabled")
	}
	if cfg.ResourceGovernor.Hysteresis.CooldownSeconds != 600 {
		t.Fatalf("governor cooldown = %d, want 600", cfg.ResourceGovernor.Hysteresis.CooldownSeconds)
	}
	if cfg.ResourceGovernor.Levels.Attack.ConntrackPercent != 75 {
		t.Fatalf("governor attack conntrack = %.1f, want 75.0", cfg.ResourceGovernor.Levels.Attack.ConntrackPercent)
	}
	if !cfg.Updates.Enabled || cfg.Updates.Channel != "stable" || cfg.Updates.ManifestURL == "" || !cfg.Updates.RequireSignedManifest {
		t.Fatalf("unexpected runtime update config: %#v", cfg.Updates)
	}
	if !cfg.RuntimeSecurity.Enabled || !cfg.RuntimeSecurity.FileIntegrityEnabled || len(cfg.RuntimeSecurity.FileIntegrityPaths) == 0 {
		t.Fatalf("unexpected runtime security config: %#v", cfg.RuntimeSecurity)
	}
	if !cfg.Telemetry.HealthReportEnabled || !cfg.Telemetry.Privacy.RedactSecrets || cfg.Telemetry.Privacy.SendCookie {
		t.Fatalf("unexpected telemetry privacy config: %#v", cfg.Telemetry)
	}
}

func TestLoadRuntimeMapsTenantAutoAttackModeToGovernor(t *testing.T) {
	cfg, err := LoadRuntimeFile("../../../configs/tenant.server-only.example.yaml")
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	if !cfg.ResourceGovernor.Enabled {
		t.Fatal("expected tenant auto_attack_mode to enable resource governor")
	}
	if cfg.WAF.Enabled {
		t.Fatalf("server-only tenant should not enable WAF: %#v", cfg.WAF)
	}
	if cfg.ResourceGovernor.Baseline.StoreFile != "/var/lib/kiro/baseline/baseline.json" {
		t.Fatalf("baseline store = %q", cfg.ResourceGovernor.Baseline.StoreFile)
	}
	if !cfg.Updates.Enabled || cfg.Updates.Channel != "stable" || !cfg.Updates.RequireSignedManifest {
		t.Fatalf("unexpected tenant update config: %#v", cfg.Updates)
	}
}

func TestLoadRuntimeMapsTenantWebDefenseDefaults(t *testing.T) {
	cfg, err := LoadRuntimeFile("../../../configs/tenant.full-cloudflare.example.yaml")
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	if !cfg.WAF.Enabled || cfg.WAF.Engine != "coraza" || cfg.WAF.AnomalyThreshold != 5 {
		t.Fatalf("unexpected tenant WAF config: %#v", cfg.WAF)
	}
	if !cfg.Bot.Enabled || !cfg.Bot.CookieChallenge || cfg.Bot.ChallengeCookieName != "kiro_challenge" {
		t.Fatalf("unexpected tenant bot config: %#v", cfg.Bot)
	}
	if len(cfg.Bot.TrustedClientCIDRs) != 1 || cfg.Bot.TrustedClientCIDRs[0] != "203.0.113.10/32" {
		t.Fatalf("trusted client CIDRs = %#v", cfg.Bot.TrustedClientCIDRs)
	}
}

func TestLoadRuntimeRejectsProviderConfig(t *testing.T) {
	if _, err := LoadRuntimeFile("../../../configs/provider.example.yaml"); err == nil {
		t.Fatal("expected provider config runtime expansion to fail")
	}
}

func TestValidateRuntimeRejectsDuplicateDomains(t *testing.T) {
	cfg := RuntimeConfig{
		Mode: "full",
		BackendPools: []RuntimeBackendPool{{
			ID: "pool",
			Upstreams: []RuntimeUpstream{{
				ID:  "upstream",
				URL: "http://127.0.0.1:3000",
			}},
		}},
		Sites: []RuntimeSite{
			{ID: "site_1", Domains: []string{"example.com"}, DefaultBackendPool: "pool"},
			{ID: "site_2", Domains: []string{"example.com"}, DefaultBackendPool: "pool"},
		},
	}
	if err := ValidateRuntime(cfg); err == nil {
		t.Fatal("expected duplicate domain to fail")
	}
}

func TestValidateAdvancedRejectsUnknownRouteBackendPool(t *testing.T) {
	cfg := AdvancedConfig{
		Mode:              "full",
		DeploymentProfile: "school_smb",
		NodeRole:          "protected_server",
		BackendPools: []BackendPool{{
			ID:        "known",
			Upstreams: []Upstream{{ID: "upstream", URL: "http://127.0.0.1:3000"}},
		}},
		Sites: []AdvancedSite{{
			ID:                 "site",
			Domains:            []string{"example.com"},
			DefaultBackendPool: "known",
			Routes:             []AdvancedRoute{{Path: "/api/", BackendPool: "missing"}},
		}},
	}
	if err := ValidateAdvanced(cfg); err == nil {
		t.Fatal("expected missing route backend pool to fail")
	}
}

func TestValidateAdvancedRequiresLicenseFilesWhenEnforced(t *testing.T) {
	cfg := AdvancedConfig{
		Mode:              "server",
		DeploymentProfile: "school_smb",
		NodeRole:          "protected_server",
		License:           AdvancedLicense{RequireValidLicense: true},
		BackendPools: []BackendPool{{
			ID:        "known",
			Upstreams: []Upstream{{ID: "upstream", URL: "http://127.0.0.1:3000"}},
		}},
	}
	if err := ValidateAdvanced(cfg); err == nil {
		t.Fatal("expected missing enforced license files to fail")
	}
}

func TestValidateAdvancedRejectsInvalidXDPMode(t *testing.T) {
	cfg := AdvancedConfig{
		Mode:              "server",
		DeploymentProfile: "school_smb",
		NodeRole:          "protected_server",
		ServerProtection: ServerProtection{
			XDP: AdvancedXDP{Enabled: true, Mode: "bad"},
		},
		BackendPools: []BackendPool{{
			ID:        "known",
			Upstreams: []Upstream{{ID: "upstream", URL: "http://127.0.0.1:3000"}},
		}},
	}
	if err := ValidateAdvanced(cfg); err == nil {
		t.Fatal("expected invalid XDP mode to fail")
	}
}

func TestTenantRejectsInvalidProfile(t *testing.T) {
	cfg := TenantConfig{
		Mode:       "server",
		Plan:       "school_smb",
		LicenseKey: "KIRO-TEST",
		Admin:      TenantAdmin{AllowIPs: []string{"203.0.113.10/32"}},
		Server:     TenantServer{Interface: "eth0", SSHPort: 22},
		Protection: TenantProtection{Profile: "invalid"},
	}
	if err := ValidateTenant(cfg); err == nil {
		t.Fatal("expected invalid profile to fail")
	}
}

func TestCheckFileRejectsMalformedYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte("mode: full\n  bad"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if _, err := CheckFile(path); err == nil {
		t.Fatal("expected malformed yaml to fail")
	}
}

func TestTenantRejectsMissingBackend(t *testing.T) {
	cfg := TenantConfig{
		Mode:       "full",
		Plan:       "school_smb",
		LicenseKey: "KIRO-TEST",
		Admin:      TenantAdmin{AllowIPs: []string{"203.0.113.10/32"}},
		Server:     TenantServer{Interface: "eth0", SSHPort: 22},
		Protection: TenantProtection{Profile: "balanced"},
		Website: TenantWebsite{
			Enabled: true,
			TLSMode: "flexible_http",
			Sites: []TenantSite{{
				Domains: []string{"example.com"},
			}},
		},
	}
	if err := ValidateTenant(cfg); err == nil {
		t.Fatal("expected missing backend to fail")
	}
}

func TestTenantRejectsDuplicateDomains(t *testing.T) {
	cfg := TenantConfig{
		Mode:       "full",
		Plan:       "school_smb",
		LicenseKey: "KIRO-TEST",
		Admin:      TenantAdmin{AllowIPs: []string{"203.0.113.10/32"}},
		Server:     TenantServer{Interface: "eth0", SSHPort: 22},
		Protection: TenantProtection{Profile: "balanced"},
		Website: TenantWebsite{
			Enabled: true,
			TLSMode: "flexible_http",
			Sites: []TenantSite{
				{Domains: []string{"example.com"}, Backend: "http://127.0.0.1:3000"},
				{Domains: []string{"example.com"}, Backend: "http://127.0.0.1:3001"},
			},
		},
	}
	if err := ValidateTenant(cfg); err == nil {
		t.Fatal("expected duplicate domain to fail")
	}
}
