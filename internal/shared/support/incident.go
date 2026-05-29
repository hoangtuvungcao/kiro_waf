package support

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

type IncidentOptions struct {
	OutputDir        string
	ConfigPath       string
	Runtime          config.RuntimeConfig
	IncidentID       string
	Type             string
	Severity         string
	Status           string
	Summary          string
	StartedAt        time.Time
	DetectedAt       time.Time
	SupportBundleDir string
	HealthPath       string
	AlertsPath       string
	Now              time.Time
}

type IncidentResult struct {
	Directory    string `json:"directory"`
	JSONPath     string `json:"json_path"`
	MarkdownPath string `json:"markdown_path"`
	IncidentID   string `json:"incident_id"`
}

type IncidentReport struct {
	IncidentID       string              `json:"incident_id"`
	GeneratedAt      string              `json:"generated_at"`
	Version          string              `json:"version"`
	Type             string              `json:"type"`
	Severity         string              `json:"severity"`
	Status           string              `json:"status"`
	Summary          string              `json:"summary"`
	StartedAt        string              `json:"started_at,omitempty"`
	DetectedAt       string              `json:"detected_at,omitempty"`
	Mode             string              `json:"mode"`
	Plan             string              `json:"plan"`
	SourceKind       string              `json:"source_kind"`
	SupportBundleDir string              `json:"support_bundle_dir,omitempty"`
	EvidenceFiles    []string            `json:"evidence_files"`
	Checklist        []IncidentChecklist `json:"checklist"`
}

type IncidentChecklist struct {
	Step   string `json:"step"`
	Status string `json:"status"`
}

var incidentIDSanitizer = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)

func BuildIncidentReport(opts IncidentOptions) (IncidentResult, error) {
	if strings.TrimSpace(opts.OutputDir) == "" {
		return IncidentResult{}, errors.New("incident output directory is required")
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}
	incidentType := firstNonEmptyIncident(opts.Type, "other")
	incidentID := firstNonEmptyIncident(opts.IncidentID, defaultIncidentID(incidentType, opts.Now))
	outputDir := filepath.Join(opts.OutputDir, incidentID)
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return IncidentResult{}, err
	}
	report := IncidentReport{
		IncidentID:       incidentID,
		GeneratedAt:      opts.Now.Format(time.RFC3339),
		Version:          buildinfo.Version,
		Type:             incidentType,
		Severity:         firstNonEmptyIncident(opts.Severity, "medium"),
		Status:           firstNonEmptyIncident(opts.Status, "open"),
		Summary:          firstNonEmptyIncident(opts.Summary, "incident summary pending"),
		Mode:             opts.Runtime.Mode,
		Plan:             opts.Runtime.Plan,
		SourceKind:       string(opts.Runtime.SourceKind),
		SupportBundleDir: strings.TrimSpace(opts.SupportBundleDir),
		EvidenceFiles:    incidentEvidence(opts),
		Checklist:        incidentChecklist(incidentType),
	}
	if !opts.StartedAt.IsZero() {
		report.StartedAt = opts.StartedAt.UTC().Format(time.RFC3339)
	}
	if !opts.DetectedAt.IsZero() {
		report.DetectedAt = opts.DetectedAt.UTC().Format(time.RFC3339)
	}
	jsonPath := filepath.Join(outputDir, "incident-report.json")
	if err := storage.WriteJSONAtomic(jsonPath, report); err != nil {
		return IncidentResult{}, err
	}
	markdownPath := filepath.Join(outputDir, "incident-report.md")
	if err := os.WriteFile(markdownPath, []byte(RenderIncidentMarkdown(report)), 0o640); err != nil {
		return IncidentResult{}, err
	}
	return IncidentResult{
		Directory:    outputDir,
		JSONPath:     jsonPath,
		MarkdownPath: markdownPath,
		IncidentID:   incidentID,
	}, nil
}

func RenderIncidentMarkdown(report IncidentReport) string {
	var b strings.Builder
	writeMarkdownLine(&b, "# Incident Report")
	writeMarkdownLine(&b, "")
	writeMarkdownLine(&b, "- Incident ID: "+report.IncidentID)
	writeMarkdownLine(&b, "- Generated At: "+report.GeneratedAt)
	writeMarkdownLine(&b, "- Version: "+report.Version)
	writeMarkdownLine(&b, "- Type: "+report.Type)
	writeMarkdownLine(&b, "- Severity: "+report.Severity)
	writeMarkdownLine(&b, "- Status: "+report.Status)
	writeMarkdownLine(&b, "- Mode: "+report.Mode)
	writeMarkdownLine(&b, "- Plan: "+report.Plan)
	if report.StartedAt != "" {
		writeMarkdownLine(&b, "- Started At: "+report.StartedAt)
	}
	if report.DetectedAt != "" {
		writeMarkdownLine(&b, "- Detected At: "+report.DetectedAt)
	}
	writeMarkdownLine(&b, "")
	writeMarkdownLine(&b, "## Summary")
	writeMarkdownLine(&b, "")
	writeMarkdownLine(&b, RedactText(report.Summary))
	writeMarkdownLine(&b, "")
	writeMarkdownLine(&b, "## Evidence")
	writeMarkdownLine(&b, "")
	if report.SupportBundleDir != "" {
		writeMarkdownLine(&b, "- Support bundle: "+report.SupportBundleDir)
	}
	for _, file := range report.EvidenceFiles {
		writeMarkdownLine(&b, "- "+file)
	}
	writeMarkdownLine(&b, "")
	writeMarkdownLine(&b, "## Checklist")
	writeMarkdownLine(&b, "")
	for _, item := range report.Checklist {
		writeMarkdownLine(&b, "- ["+item.Status+"] "+item.Step)
	}
	writeMarkdownLine(&b, "")
	writeMarkdownLine(&b, "## Timeline")
	writeMarkdownLine(&b, "")
	writeMarkdownLine(&b, "- Add operator timeline here.")
	return b.String()
}

func incidentEvidence(opts IncidentOptions) []string {
	var evidence []string
	for _, path := range []string{opts.ConfigPath, opts.HealthPath, opts.AlertsPath} {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			evidence = append(evidence, path)
		}
	}
	if opts.SupportBundleDir != "" {
		summaryPath := filepath.Join(opts.SupportBundleDir, "summary.json")
		if _, err := os.Stat(summaryPath); err == nil {
			evidence = append(evidence, summaryPath)
		}
	}
	return evidence
}

func incidentChecklist(incidentType string) []IncidentChecklist {
	switch strings.TrimSpace(incidentType) {
	case "attack":
		return checklist(
			"preserve support bundle and current health report",
			"confirm admin access path before any firewall apply",
			"review governor level, WAF/bot decisions, firewall snapshot, and Cloudflare origin lock",
			"record mitigation actions and cooldown time",
			"do not run public attack traffic tests",
		)
	case "lost_ssh":
		return checklist(
			"use VPS/provider console or rescue access",
			"check pending firewall rollback state",
			"run firewall rollback or restore last-good config",
			"verify admin CIDR and SSH port before re-apply",
			"capture root cause before closing incident",
		)
	case "update_failed":
		return checklist(
			"do not confirm pending update",
			"run update rollback while pending state exists",
			"verify manifest signature, artifact checksum, and release metadata",
			"collect health report and support bundle after rollback",
			"publish corrected release or hold channel manifest",
		)
	case "origin_ip_leaked":
		return checklist(
			"confirm DNS records are proxied through Cloudflare",
			"enable or verify Cloudflare origin lock rules",
			"block direct origin HTTP/HTTPS where required",
			"rotate origin IP if exposure risk remains",
			"record leak source and customer-facing impact",
		)
	case "license_rebind":
		return checklist(
			"collect new machine fingerprint hash",
			"verify current license status and rebind quota",
			"run provider rebind workflow with reason",
			"export only license.json and provider public key to agent",
			"verify license locally on protected server",
		)
	case "runtime_security":
		return checklist(
			"preserve runtime alerts and file integrity snapshots",
			"isolate affected web application process if needed",
			"compare current files with baseline",
			"rotate application secrets outside support bundle",
			"document containment and recovery steps",
		)
	default:
		return checklist(
			"collect support bundle",
			"collect health report",
			"record timeline",
			"document mitigation and rollback actions",
		)
	}
}

func checklist(steps ...string) []IncidentChecklist {
	items := make([]IncidentChecklist, 0, len(steps))
	for _, step := range steps {
		items = append(items, IncidentChecklist{Step: step, Status: " "})
	}
	return items
}

func defaultIncidentID(incidentType string, now time.Time) string {
	raw := fmt.Sprintf("inc_%s_%s", now.UTC().Format("20060102T150405Z"), incidentType)
	return incidentIDSanitizer.ReplaceAllString(raw, "_")
}

func writeMarkdownLine(b *strings.Builder, line string) {
	b.WriteString(line)
	b.WriteByte('\n')
}

func firstNonEmptyIncident(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
