package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExportProviderInboxRedactsAndIndexesFiles(t *testing.T) {
	dir := t.TempDir()
	health := filepath.Join(dir, "health.json")
	alerts := filepath.Join(dir, "alerts.jsonl")
	bundle := filepath.Join(dir, "bundle")
	incident := filepath.Join(dir, "incident")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	if err := os.MkdirAll(incident, 0o755); err != nil {
		t.Fatalf("mkdir incident: %v", err)
	}
	writeSupportFixture(t, health, `{"status":"pass","api_token":"secret"}`)
	writeSupportFixture(t, alerts, `{"type":"process_exec","cookie":"session=secret"}`)
	writeSupportFixture(t, filepath.Join(bundle, "summary.json"), `{"mode":"full"}`)
	writeSupportFixture(t, filepath.Join(incident, "incident-report.md"), "Authorization: Bearer secret-token\n")
	writeSupportFixture(t, filepath.Join(incident, "incident-report.json"), `{"summary":"ok"}`)

	result, err := ExportProviderInbox(ProviderExportOptions{
		OutputDir:        filepath.Join(dir, "provider-inbox"),
		ServerID:         "srv_001",
		HealthPath:       health,
		AlertsPath:       alerts,
		SupportBundleDir: bundle,
		IncidentDir:      incident,
		Now:              time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("export provider inbox: %v", err)
	}
	if result.IndexPath == "" || len(result.Files) < 5 {
		t.Fatalf("unexpected result: %#v", result)
	}
	for _, name := range []string{"health-report.json", "runtime-alerts.jsonl", "incident-report.md"} {
		raw, err := os.ReadFile(filepath.Join(result.Directory, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.Contains(string(raw), "session=secret") ||
			strings.Contains(string(raw), "secret-token") ||
			strings.Contains(string(raw), `"api_token"`) ||
			strings.Contains(string(raw), `"cookie"`) {
			t.Fatalf("%s was not redacted: %s", name, raw)
		}
	}
}

func writeSupportFixture(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
