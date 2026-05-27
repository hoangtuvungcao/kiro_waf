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
