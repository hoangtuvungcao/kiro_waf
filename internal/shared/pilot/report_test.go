package pilot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kiro_waf/internal/shared/config"
	"kiro_waf/internal/shared/storage"
)

func TestBuildReportReturnsGoWhenEvidenceIsComplete(t *testing.T) {
	dir := t.TempDir()
	opts := completeOptions(t, dir)
	result, err := BuildReport(opts)
	if err != nil {
		t.Fatalf("build report: %v", err)
	}
	if result.Decision != "go" {
		t.Fatalf("decision = %s", result.Decision)
	}
	var report Report
	if err := storage.ReadJSON(result.JSONPath, &report); err != nil {
		t.Fatalf("read report: %v", err)
	}
	if report.Decision != "go" || len(report.Blockers) != 0 {
		t.Fatalf("unexpected report: %#v", report)
	}
	raw, err := os.ReadFile(result.MarkdownPath)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	if !strings.Contains(string(raw), "Decision: GO") {
		t.Fatalf("markdown missing decision:\n%s", string(raw))
	}
}

func TestBuildReportHoldsWhenEvidenceMissing(t *testing.T) {
	dir := t.TempDir()
	opts := completeOptions(t, dir)
	opts.RevocationPath = filepath.Join(dir, "missing-revocations.json")
	result, err := BuildReport(opts)
	if err != nil {
		t.Fatalf("build report: %v", err)
	}
	if result.Decision != "hold" {
		t.Fatalf("decision = %s", result.Decision)
	}
	var report Report
	if err := storage.ReadJSON(result.JSONPath, &report); err != nil {
		t.Fatalf("read report: %v", err)
	}
	if len(report.Blockers) == 0 {
		t.Fatalf("expected blockers: %#v", report)
	}
}

func completeOptions(t *testing.T, dir string) Options {
	t.Helper()
	files := map[string]string{
		"config.yaml":             "mode: full\n",
		"health.json":             "{}\n",
		"benchmark.json":          "{}\n",
		"benchmark-evidence.json": "{}\n",
		"update-evidence.txt":     "update confirmed\n",
		"revocations-synced.json": "{}\n",
		"proxy-nginx.conf":        "modsecurity on;\n",
	}
	paths := map[string]string{}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		paths[name] = path
	}
	incidentDir := filepath.Join(dir, "incident")
	if err := os.MkdirAll(incidentDir, 0o755); err != nil {
		t.Fatalf("mkdir incident: %v", err)
	}
	return Options{
		OutputDir:             filepath.Join(dir, "pilot-out"),
		ConfigPath:            paths["config.yaml"],
		Runtime:               config.RuntimeConfig{Mode: "full", Plan: "school_smb", SourceKind: config.KindAdvanced},
		PilotID:               "pilot_ci",
		ServerCount:           3,
		StartedAt:             time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC),
		HealthPath:            paths["health.json"],
		BenchmarkPath:         paths["benchmark.json"],
		BenchmarkEvidencePath: paths["benchmark-evidence.json"],
		IncidentDir:           incidentDir,
		UpdateEvidencePath:    paths["update-evidence.txt"],
		RevocationPath:        paths["revocations-synced.json"],
		ProxyEvidencePath:     paths["proxy-nginx.conf"],
		Now:                   time.Date(2026, 5, 28, 1, 0, 0, 0, time.UTC),
	}
}
