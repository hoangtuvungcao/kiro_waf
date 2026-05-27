package config

import "testing"

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
