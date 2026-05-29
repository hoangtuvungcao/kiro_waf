package pilot

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"kiro_waf/internal/shared/buildinfo"
	"kiro_waf/internal/shared/config"
	"kiro_waf/internal/shared/storage"
)

type Options struct {
	OutputDir             string
	ConfigPath            string
	Runtime               config.RuntimeConfig
	PilotID               string
	ServerCount           int
	StartedAt             time.Time
	EndedAt               time.Time
	HealthPath            string
	BenchmarkPath         string
	BenchmarkEvidencePath string
	IncidentDir           string
	UpdateEvidencePath    string
	RevocationPath        string
	ProxyEvidencePath     string
	Now                   time.Time
}

type Result struct {
	Directory    string `json:"directory"`
	JSONPath     string `json:"json_path"`
	MarkdownPath string `json:"markdown_path"`
	PilotID      string `json:"pilot_id"`
	Decision     string `json:"decision"`
}

type Report struct {
	PilotID      string          `json:"pilot_id"`
	GeneratedAt  string          `json:"generated_at"`
	Version      string          `json:"version"`
	Mode         string          `json:"mode"`
	Plan         string          `json:"plan"`
	SourceKind   string          `json:"source_kind"`
	ServerCount  int             `json:"server_count"`
	StartedAt    string          `json:"started_at,omitempty"`
	EndedAt      string          `json:"ended_at,omitempty"`
	DurationDays int             `json:"duration_days"`
	Decision     string          `json:"decision"`
	Evidence     []EvidenceItem  `json:"evidence"`
	Checklist    []ChecklistItem `json:"checklist"`
	Blockers     []string        `json:"blockers,omitempty"`
	Notes        []string        `json:"notes"`
}

type EvidenceItem struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Path   string `json:"path,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type ChecklistItem struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

var pilotIDSanitizer = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)

func BuildReport(opts Options) (Result, error) {
	if strings.TrimSpace(opts.OutputDir) == "" {
		return Result{}, errors.New("pilot output directory is required")
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}
	if opts.EndedAt.IsZero() {
		opts.EndedAt = opts.Now
	}
	pilotID := strings.TrimSpace(opts.PilotID)
	if pilotID == "" {
		pilotID = defaultPilotID(opts.Now)
	}
	pilotID = pilotIDSanitizer.ReplaceAllString(pilotID, "_")
	outputDir := filepath.Join(opts.OutputDir, pilotID)
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return Result{}, err
	}
	report := buildReportPayload(opts, pilotID)
	jsonPath := filepath.Join(outputDir, "pilot-go-no-go.json")
	if err := storage.WriteJSONAtomic(jsonPath, report); err != nil {
		return Result{}, err
	}
	markdownPath := filepath.Join(outputDir, "pilot-go-no-go.md")
	if err := os.WriteFile(markdownPath, []byte(RenderMarkdown(report)), 0o640); err != nil {
		return Result{}, err
	}
	return Result{
		Directory:    outputDir,
		JSONPath:     jsonPath,
		MarkdownPath: markdownPath,
		PilotID:      pilotID,
		Decision:     report.Decision,
	}, nil
}

func buildReportPayload(opts Options, pilotID string) Report {
	evidence := []EvidenceItem{
		fileEvidence("config", opts.ConfigPath, "runtime config used for pilot"),
		fileEvidence("health_report", opts.HealthPath, "latest protected-server health report"),
		fileEvidence("benchmark_report", opts.BenchmarkPath, "safe local benchmark report"),
		fileEvidence("benchmark_evidence", opts.BenchmarkEvidencePath, "benchmark evidence checklist"),
		dirEvidence("incident_drill", opts.IncidentDir, "incident report directory from pilot drill"),
		fileEvidence("update_rollback_drill", opts.UpdateEvidencePath, "update apply/rollback or confirm evidence"),
		fileEvidence("revocation_sync_drill", opts.RevocationPath, "synced signed revocation list"),
		fileEvidence("proxy_waf_bot_drill", opts.ProxyEvidencePath, "generated proxy/WAF/Bot plan evidence"),
	}
	durationDays := pilotDurationDays(opts.StartedAt, opts.EndedAt)
	checks := []ChecklistItem{
		{
			Name:   "pilot_server_count",
			Status: passFail(opts.ServerCount >= 3 && opts.ServerCount <= 5),
			Detail: fmt.Sprintf("%d servers; target is 3-5", opts.ServerCount),
		},
		{
			Name:   "pilot_duration",
			Status: passFail(durationDays >= 30),
			Detail: fmt.Sprintf("%d days; target is at least 30", durationDays),
		},
		{
			Name:   "update_rollback_drill",
			Status: evidenceStatus(evidence, "update_rollback_drill"),
			Detail: "update path has collected evidence",
		},
		{
			Name:   "incident_drill",
			Status: evidenceStatus(evidence, "incident_drill"),
			Detail: "incident report exists",
		},
		{
			Name:   "revocation_drill",
			Status: evidenceStatus(evidence, "revocation_sync_drill"),
			Detail: "revocation sync evidence exists",
		},
		{
			Name:   "waf_false_positive_drill",
			Status: evidenceStatus(evidence, "proxy_waf_bot_drill"),
			Detail: "proxy/WAF/Bot evidence exists",
		},
	}
	blockers := reportBlockers(evidence, checks)
	decision := "go"
	if len(blockers) > 0 {
		decision = "hold"
	}
	report := Report{
		PilotID:      pilotID,
		GeneratedAt:  opts.Now.Format(time.RFC3339),
		Version:      buildinfo.Version,
		Mode:         opts.Runtime.Mode,
		Plan:         opts.Runtime.Plan,
		SourceKind:   string(opts.Runtime.SourceKind),
		ServerCount:  opts.ServerCount,
		DurationDays: durationDays,
		Decision:     decision,
		Evidence:     evidence,
		Checklist:    checks,
		Blockers:     blockers,
		Notes: []string{
			"Decision is evidence-based; missing drill evidence keeps the release candidate on hold.",
			"Public DDoS capacity claims still require isolated benchmark evidence by VPS size.",
		},
	}
	if !opts.StartedAt.IsZero() {
		report.StartedAt = opts.StartedAt.UTC().Format(time.RFC3339)
	}
	if !opts.EndedAt.IsZero() {
		report.EndedAt = opts.EndedAt.UTC().Format(time.RFC3339)
	}
	return report
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	writeLine(&b, "# Pilot Go/No-Go Report")
	writeLine(&b, "")
	writeLine(&b, "- Pilot ID: "+report.PilotID)
	writeLine(&b, "- Generated At: "+report.GeneratedAt)
	writeLine(&b, "- Version: "+report.Version)
	writeLine(&b, "- Mode: "+report.Mode)
	writeLine(&b, "- Plan: "+report.Plan)
	writeLine(&b, fmt.Sprintf("- Server Count: %d", report.ServerCount))
	writeLine(&b, fmt.Sprintf("- Duration Days: %d", report.DurationDays))
	writeLine(&b, "- Decision: "+strings.ToUpper(report.Decision))
	writeLine(&b, "")
	writeLine(&b, "## Evidence")
	writeLine(&b, "")
	for _, item := range report.Evidence {
		line := fmt.Sprintf("- [%s] %s", item.Status, item.Name)
		if item.Path != "" {
			line += ": " + item.Path
		}
		if item.Detail != "" {
			line += " - " + item.Detail
		}
		writeLine(&b, line)
	}
	writeLine(&b, "")
	writeLine(&b, "## Checklist")
	writeLine(&b, "")
	for _, item := range report.Checklist {
		writeLine(&b, fmt.Sprintf("- [%s] %s - %s", item.Status, item.Name, item.Detail))
	}
	if len(report.Blockers) > 0 {
		writeLine(&b, "")
		writeLine(&b, "## Blockers")
		writeLine(&b, "")
		for _, blocker := range report.Blockers {
			writeLine(&b, "- "+blocker)
		}
	}
	return b.String()
}

func fileEvidence(name string, path string, detail string) EvidenceItem {
	item := EvidenceItem{Name: name, Status: "missing", Path: strings.TrimSpace(path), Detail: detail}
	if item.Path == "" {
		return item
	}
	if info, err := os.Stat(item.Path); err == nil && !info.IsDir() {
		item.Status = "present"
	}
	return item
}

func dirEvidence(name string, path string, detail string) EvidenceItem {
	item := EvidenceItem{Name: name, Status: "missing", Path: strings.TrimSpace(path), Detail: detail}
	if item.Path == "" {
		return item
	}
	if info, err := os.Stat(item.Path); err == nil && info.IsDir() {
		item.Status = "present"
	}
	return item
}

func pilotDurationDays(startedAt time.Time, endedAt time.Time) int {
	if startedAt.IsZero() || endedAt.IsZero() || endedAt.Before(startedAt) {
		return 0
	}
	return int(endedAt.Sub(startedAt).Hours() / 24)
}

func passFail(ok bool) string {
	if ok {
		return "pass"
	}
	return "fail"
}

func evidenceStatus(items []EvidenceItem, name string) string {
	for _, item := range items {
		if item.Name == name {
			if item.Status == "present" {
				return "pass"
			}
			return "fail"
		}
	}
	return "fail"
}

func reportBlockers(evidence []EvidenceItem, checks []ChecklistItem) []string {
	var blockers []string
	for _, item := range evidence {
		if item.Status != "present" {
			blockers = append(blockers, "missing evidence: "+item.Name)
		}
	}
	for _, item := range checks {
		if item.Status != "pass" {
			blockers = append(blockers, "failed check: "+item.Name)
		}
	}
	return blockers
}

func defaultPilotID(now time.Time) string {
	return "pilot_" + now.UTC().Format("20060102T150405Z")
}

func writeLine(b *strings.Builder, line string) {
	b.WriteString(line)
	b.WriteByte('\n')
}
