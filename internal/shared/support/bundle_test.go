package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kiro_waf/internal/shared/config"
)

func TestRedactTextRemovesSecrets(t *testing.T) {
	input := "password: hunter2\napi_token: abc123\nAuthorization: Bearer secret-token\nsafe: value\n"
	output := RedactText(input)
	for _, forbidden := range []string{"hunter2", "abc123", "secret-token", "password", "token", "Authorization"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("redacted output still contains %q:\n%s", forbidden, output)
		}
	}
	if !strings.Contains(output, "safe: value") {
		t.Fatalf("safe value should remain:\n%s", output)
	}
}

func TestBuildBundleRedactsConfigAndIncludesSummary(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "kiro.yaml")
	if err := os.WriteFile(configPath, []byte("mode: full\npassword: hunter2\nlicense_key: KIRO-secret\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	alertsPath := filepath.Join(dir, "alerts.jsonl")
	if err := os.WriteFile(alertsPath, []byte(`{"type":"process_exec","cookie":"session=secret"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write alerts: %v", err)
	}
	out := filepath.Join(dir, "bundle")
	result, err := BuildBundle(BundleOptions{
		OutputDir:  out,
		ConfigPath: configPath,
		AlertsPath: alertsPath,
		Runtime: config.RuntimeConfig{
			SourceKind: config.KindAdvanced,
			Mode:       "full",
			Plan:       "school_smb",
			RuntimeSecurity: config.RuntimeSecurity{
				Enabled: true,
			},
			Telemetry: config.RuntimeTelemetry{
				Privacy: config.RuntimePrivacy{RedactSecrets: true},
			},
		},
		Now: time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("build bundle: %v", err)
	}
	if len(result.Files) < 3 {
		t.Fatalf("bundle files = %#v", result.Files)
	}
	for _, name := range []string{"config-redacted.yaml", "runtime-alerts.jsonl"} {
		raw, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		text := string(raw)
		for _, forbidden := range []string{"hunter2", "KIRO-secret", "session=secret", "password", "license_key", "cookie"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s still contains %q:\n%s", name, forbidden, text)
			}
		}
	}
	summary, err := DecodeSummary(filepath.Join(out, "summary.json"))
	if err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary.Mode != "full" || !summary.PrivacyRedaction {
		t.Fatalf("unexpected summary: %#v", summary)
	}
}
