package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kiro_waf/internal/shared/config"
)

func TestBuildIncidentReportWritesJSONAndMarkdown(t *testing.T) {
	dir := t.TempDir()
	healthPath := filepath.Join(dir, "health.json")
	alertsPath := filepath.Join(dir, "alerts.jsonl")
	if err := os.WriteFile(healthPath, []byte(`{"status":"warn"}`), 0o644); err != nil {
		t.Fatalf("write health: %v", err)
	}
	if err := os.WriteFile(alertsPath, []byte(`{"type":"process_exec"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write alerts: %v", err)
	}
	result, err := BuildIncidentReport(IncidentOptions{
		OutputDir:  filepath.Join(dir, "incidents"),
		Runtime:    testIncidentRuntime(),
		Type:       "attack",
		Severity:   "high",
		Summary:    "password: hunter2\nlarge traffic spike",
		HealthPath: healthPath,
		AlertsPath: alertsPath,
		Now:        time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("build incident report: %v", err)
	}
	if result.IncidentID != "inc_20260528T000000Z_attack" {
		t.Fatalf("incident id = %s", result.IncidentID)
	}
	assertFileExists(t, result.JSONPath)
	raw, err := os.ReadFile(result.MarkdownPath)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	text := string(raw)
	if strings.Contains(text, "hunter2") || strings.Contains(text, "password") {
		t.Fatalf("markdown was not redacted:\n%s", text)
	}
	for _, want := range []string{"# Incident Report", "do not run public attack traffic tests", healthPath, alertsPath} {
		if !strings.Contains(text, want) {
			t.Fatalf("markdown missing %q:\n%s", want, text)
		}
	}
}

func TestBuildIncidentReportRequiresOutputDir(t *testing.T) {
	if _, err := BuildIncidentReport(IncidentOptions{Runtime: testIncidentRuntime()}); err == nil {
		t.Fatal("expected output dir error")
	}
}

func TestIncidentChecklistForUpdateFailed(t *testing.T) {
	items := incidentChecklist("update_failed")
	foundRollback := false
	for _, item := range items {
		if strings.Contains(item.Step, "update rollback") {
			foundRollback = true
		}
	}
	if !foundRollback {
		t.Fatalf("expected update rollback checklist, got %#v", items)
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func testIncidentRuntime() config.RuntimeConfig {
	return config.RuntimeConfig{
		SourceKind: config.KindTenant,
		Mode:       "full",
		Plan:       "school_smb",
	}
}
