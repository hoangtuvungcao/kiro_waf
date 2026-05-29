package support

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"kiro_waf/internal/shared/config"
	"kiro_waf/internal/shared/storage"
)

type BundleOptions struct {
	OutputDir  string
	ConfigPath string
	Runtime    config.RuntimeConfig
	AlertsPath string
	HealthPath string
	Now        time.Time
}

type BundleResult struct {
	Directory string   `json:"directory"`
	Files     []string `json:"files"`
}

type Summary struct {
	GeneratedAt      string   `json:"generated_at"`
	Mode             string   `json:"mode"`
	Plan             string   `json:"plan"`
	SourceKind       string   `json:"source_kind"`
	PrivacyRedaction bool     `json:"privacy_redaction"`
	RuntimeSecurity  bool     `json:"runtime_security"`
	TelemetryEnabled bool     `json:"telemetry_enabled"`
	IncludedFiles    []string `json:"included_files"`
}

var (
	yamlSecretLine = regexp.MustCompile(`(?i)^(\s*)[^:\n]*(password|token|secret|api[_-]?key|authorization|cookie|license_key)[^:\n]*:.*$`)
	jsonSecretPair = regexp.MustCompile(`(?i)"[^"]*(password|token|secret|api[_-]?key|authorization|cookie|license_key)[^"]*"\s*:\s*("[^"]*"|[0-9]+|true|false|null)`)
	bearerToken    = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._~+/=-]+`)
)

func BuildBundle(opts BundleOptions) (BundleResult, error) {
	if strings.TrimSpace(opts.OutputDir) == "" {
		return BundleResult{}, errors.New("support bundle output directory is required")
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}
	if err := os.MkdirAll(opts.OutputDir, 0o750); err != nil {
		return BundleResult{}, err
	}
	result := BundleResult{Directory: opts.OutputDir}

	summary := Summary{
		GeneratedAt:      opts.Now.Format(time.RFC3339),
		Mode:             opts.Runtime.Mode,
		Plan:             opts.Runtime.Plan,
		SourceKind:       string(opts.Runtime.SourceKind),
		PrivacyRedaction: opts.Runtime.Telemetry.Privacy.RedactSecrets,
		RuntimeSecurity:  opts.Runtime.RuntimeSecurity.Enabled,
		TelemetryEnabled: opts.Runtime.Telemetry.Enabled,
	}
	if !summary.PrivacyRedaction {
		summary.PrivacyRedaction = true
	}
	summaryPath := filepath.Join(opts.OutputDir, "summary.json")
	if err := storage.WriteJSONAtomic(summaryPath, summary); err != nil {
		return BundleResult{}, err
	}
	result.Files = append(result.Files, summaryPath)

	if opts.ConfigPath != "" {
		path, err := writeRedactedCopy(opts.OutputDir, "config-redacted.yaml", opts.ConfigPath)
		if err != nil {
			return BundleResult{}, err
		}
		result.Files = append(result.Files, path)
	}
	if opts.AlertsPath != "" {
		if _, err := os.Stat(opts.AlertsPath); err == nil {
			path, err := writeRedactedCopy(opts.OutputDir, "runtime-alerts.jsonl", opts.AlertsPath)
			if err != nil {
				return BundleResult{}, err
			}
			result.Files = append(result.Files, path)
		}
	}
	if opts.HealthPath != "" {
		if _, err := os.Stat(opts.HealthPath); err == nil {
			path, err := writeRedactedCopy(opts.OutputDir, "health-report.json", opts.HealthPath)
			if err != nil {
				return BundleResult{}, err
			}
			result.Files = append(result.Files, path)
		}
	}

	summary.IncludedFiles = baseNames(result.Files)
	if err := storage.WriteJSONAtomic(summaryPath, summary); err != nil {
		return BundleResult{}, err
	}
	return result, nil
}

func RedactText(input string) string {
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		if yamlSecretLine.MatchString(line) {
			indent := leadingWhitespace(line)
			lines[i] = indent + "redacted_secret: <redacted>"
		}
	}
	out := strings.Join(lines, "\n")
	out = jsonSecretPair.ReplaceAllString(out, `"redacted_secret":"<redacted>"`)
	out = bearerToken.ReplaceAllString(out, "Bearer <redacted>")
	return out
}

func writeRedactedCopy(outputDir string, name string, source string) (string, error) {
	raw, err := os.ReadFile(source)
	if err != nil {
		return "", err
	}
	path := filepath.Join(outputDir, name)
	if err := os.WriteFile(path, []byte(RedactText(string(raw))), 0o640); err != nil {
		return "", err
	}
	return path, nil
}

func baseNames(paths []string) []string {
	names := make([]string, 0, len(paths))
	for _, path := range paths {
		names = append(names, filepath.Base(path))
	}
	return names
}

func leadingWhitespace(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r != ' ' && r != '\t' {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}

func DecodeSummary(path string) (Summary, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Summary{}, err
	}
	var summary Summary
	if err := json.Unmarshal(raw, &summary); err != nil {
		return Summary{}, err
	}
	return summary, nil
}
