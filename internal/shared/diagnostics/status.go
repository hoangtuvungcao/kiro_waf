package diagnostics

import (
	"encoding/json"
	"time"

	"kiro_waf/internal/shared/buildinfo"
	"kiro_waf/internal/shared/config"
)

type StatusReport struct {
	GeneratedAt       string `json:"generated_at"`
	Version           string `json:"version"`
	SourceKind        string `json:"source_kind"`
	Mode              string `json:"mode"`
	Plan              string `json:"plan"`
	Sites             int    `json:"sites"`
	BackendPools      int    `json:"backend_pools"`
	FirewallEnabled   bool   `json:"firewall_enabled"`
	ProxyEnabled      bool   `json:"proxy_enabled"`
	CloudflareLock    bool   `json:"cloudflare_origin_lock"`
	WAFEnabled        bool   `json:"waf_enabled"`
	BotEnabled        bool   `json:"bot_enabled"`
	GovernorEnabled   bool   `json:"governor_enabled"`
	UpdatesEnabled    bool   `json:"updates_enabled"`
	RuntimeSecurity   bool   `json:"runtime_security_enabled"`
	TelemetryEnabled  bool   `json:"telemetry_enabled"`
	StateDir          string `json:"state_dir"`
	LastGoodConfigDir string `json:"last_good_config_dir"`
}

type HealthReport struct {
	GeneratedAt string  `json:"generated_at"`
	Status      string  `json:"status"`
	Checks      []Check `json:"checks"`
}

type Check struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Message  string `json:"message"`
	Severity string `json:"severity,omitempty"`
}

func BuildStatus(runtime config.RuntimeConfig, now time.Time) StatusReport {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return StatusReport{
		GeneratedAt:       now.Format(time.RFC3339),
		Version:           buildinfo.Version,
		SourceKind:        string(runtime.SourceKind),
		Mode:              runtime.Mode,
		Plan:              runtime.Plan,
		Sites:             len(runtime.Sites),
		BackendPools:      len(runtime.BackendPools),
		FirewallEnabled:   runtime.Firewall.Enabled,
		ProxyEnabled:      runtime.Mode == "full" && len(runtime.Sites) > 0,
		CloudflareLock:    runtime.CFOriginLock.Enabled,
		WAFEnabled:        runtime.WAF.Enabled,
		BotEnabled:        runtime.Bot.Enabled,
		GovernorEnabled:   runtime.ResourceGovernor.Enabled,
		UpdatesEnabled:    runtime.Updates.Enabled,
		RuntimeSecurity:   runtime.RuntimeSecurity.Enabled,
		TelemetryEnabled:  runtime.Telemetry.Enabled,
		StateDir:          runtime.Paths.StateDir,
		LastGoodConfigDir: runtime.Paths.LastGoodConfigDir,
	}
}

func BuildHealth(runtime config.RuntimeConfig, preflight PreflightReport, now time.Time) HealthReport {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	checks := []Check{
		{Name: "config", Status: "pass", Message: "runtime config expanded successfully"},
		modeCheck(runtime),
		adminAllowlistCheck(runtime),
		firewallSafetyCheck(runtime),
		fullModeCheck(runtime),
		privacyCheck(runtime),
	}
	for _, check := range preflight.Checks {
		checks = append(checks, check)
	}
	return HealthReport{
		GeneratedAt: now.Format(time.RFC3339),
		Status:      overallStatus(checks),
		Checks:      checks,
	}
}

func MarshalJSON(value any) ([]byte, error) {
	return json.MarshalIndent(value, "", "  ")
}

func modeCheck(runtime config.RuntimeConfig) Check {
	if runtime.Mode == "server" || runtime.Mode == "full" {
		return Check{Name: "mode", Status: "pass", Message: "mode is valid"}
	}
	return Check{Name: "mode", Status: "fail", Severity: "critical", Message: "mode must be server or full"}
}

func adminAllowlistCheck(runtime config.RuntimeConfig) Check {
	if len(runtime.AdminCIDRs) > 0 || len(runtime.Firewall.AdminCIDRs) > 0 {
		return Check{Name: "admin_allowlist", Status: "pass", Message: "admin allowlist is configured"}
	}
	return Check{Name: "admin_allowlist", Status: "fail", Severity: "critical", Message: "admin allowlist is required before firewall apply"}
}

func firewallSafetyCheck(runtime config.RuntimeConfig) Check {
	if !runtime.Firewall.Enabled {
		return Check{Name: "firewall", Status: "warn", Severity: "medium", Message: "firewall is disabled"}
	}
	if runtime.Firewall.SSHAdminOnly && len(runtime.Firewall.AdminCIDRs) == 0 {
		return Check{Name: "firewall", Status: "fail", Severity: "critical", Message: "ssh_admin_only needs admin CIDRs"}
	}
	return Check{Name: "firewall", Status: "pass", Message: "firewall safety settings are present"}
}

func fullModeCheck(runtime config.RuntimeConfig) Check {
	if runtime.Mode != "full" {
		return Check{Name: "website_proxy", Status: "pass", Message: "server mode does not require website proxy"}
	}
	if len(runtime.Sites) == 0 || len(runtime.BackendPools) == 0 {
		return Check{Name: "website_proxy", Status: "fail", Severity: "critical", Message: "full mode requires sites and backend pools"}
	}
	return Check{Name: "website_proxy", Status: "pass", Message: "full mode has sites and backend pools"}
}

func privacyCheck(runtime config.RuntimeConfig) Check {
	privacy := runtime.Telemetry.Privacy
	if privacy.SendRequestBody || privacy.SendCookie || privacy.SendAuthorizationHeader {
		return Check{Name: "privacy", Status: "fail", Severity: "high", Message: "privacy config sends sensitive HTTP data"}
	}
	if !privacy.RedactSecrets {
		return Check{Name: "privacy", Status: "warn", Severity: "medium", Message: "support bundle redaction is disabled"}
	}
	return Check{Name: "privacy", Status: "pass", Message: "sensitive telemetry is disabled by default"}
}

func overallStatus(checks []Check) string {
	status := "pass"
	for _, check := range checks {
		if check.Status == "fail" {
			return "fail"
		}
		if check.Status == "warn" {
			status = "warn"
		}
	}
	return status
}
