package support

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kiro_waf/internal/shared/storage"
)

type ProviderExportOptions struct {
	OutputDir        string
	ServerID         string
	HealthPath       string
	AlertsPath       string
	SupportBundleDir string
	IncidentDir      string
	Now              time.Time
}

type ProviderExportResult struct {
	Directory string   `json:"directory"`
	IndexPath string   `json:"index_path"`
	ServerID  string   `json:"server_id"`
	Files     []string `json:"files"`
}

type ProviderExportIndex struct {
	GeneratedAt string   `json:"generated_at"`
	ServerID    string   `json:"server_id"`
	Files       []string `json:"files"`
}

func ExportProviderInbox(opts ProviderExportOptions) (ProviderExportResult, error) {
	if strings.TrimSpace(opts.OutputDir) == "" {
		return ProviderExportResult{}, errors.New("provider export output directory is required")
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}
	serverID := firstNonEmptyExport(opts.ServerID, "unknown-server")
	dir := filepath.Join(opts.OutputDir, serverID, opts.Now.Format("20060102T150405Z"))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return ProviderExportResult{}, err
	}
	result := ProviderExportResult{Directory: dir, ServerID: serverID}
	items := []struct {
		name string
		path string
	}{
		{name: "health-report.json", path: opts.HealthPath},
		{name: "runtime-alerts.jsonl", path: opts.AlertsPath},
	}
	if strings.TrimSpace(opts.SupportBundleDir) != "" {
		items = append(items, struct {
			name string
			path string
		}{name: "support-summary.json", path: filepath.Join(opts.SupportBundleDir, "summary.json")})
	}
	if strings.TrimSpace(opts.IncidentDir) != "" {
		items = append(items,
			struct {
				name string
				path string
			}{name: "incident-report.json", path: filepath.Join(opts.IncidentDir, "incident-report.json")},
			struct {
				name string
				path string
			}{name: "incident-report.md", path: filepath.Join(opts.IncidentDir, "incident-report.md")},
		)
	}
	for _, item := range items {
		if strings.TrimSpace(item.path) == "" {
			continue
		}
		if _, err := os.Stat(item.path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return ProviderExportResult{}, err
		}
		out, err := writeRedactedCopy(dir, item.name, item.path)
		if err != nil {
			return ProviderExportResult{}, err
		}
		result.Files = append(result.Files, out)
	}
	index := ProviderExportIndex{
		GeneratedAt: opts.Now.Format(time.RFC3339),
		ServerID:    serverID,
		Files:       baseNames(result.Files),
	}
	indexPath := filepath.Join(dir, "provider-inbox-index.json")
	if err := storage.WriteJSONAtomic(indexPath, index); err != nil {
		return ProviderExportResult{}, err
	}
	result.IndexPath = indexPath
	result.Files = append([]string{indexPath}, result.Files...)
	return result, nil
}

func firstNonEmptyExport(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
