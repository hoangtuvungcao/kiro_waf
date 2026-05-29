package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	allowedModes      = map[string]bool{"server": true, "full": true}
	allowedProfiles   = map[string]bool{"light": true, "balanced": true, "strict": true, "lockdown": true}
	allowedTLSModes   = map[string]bool{"flexible_http": true, "full_tls": true, "full_strict": true}
	allowedWAFEngines = map[string]bool{"coraza": true, "modsecurity": true}
)

func CheckFile(path string) (Result, error) {
	_, result, err := loadFile(path)
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

func LoadProviderFile(path string) (ProviderConfig, error) {
	cfg, result, err := loadFile(path)
	if err != nil {
		return ProviderConfig{}, err
	}
	if result.Kind != KindProvider {
		return ProviderConfig{}, fmt.Errorf("expected provider config, got %s", result.Kind)
	}
	providerCfg, ok := cfg.(ProviderConfig)
	if !ok {
		return ProviderConfig{}, errors.New("provider config type assertion failed")
	}
	return providerCfg, nil
}

func loadFile(path string) (any, Result, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, Result{}, err
	}

	var probe map[string]any
	if err := yaml.Unmarshal(raw, &probe); err != nil {
		return nil, Result{}, fmt.Errorf("parse yaml: %w", err)
	}

	if _, ok := probe["provider"]; ok {
		var cfg ProviderConfig
		if err := yaml.Unmarshal(raw, &cfg); err != nil {
			return nil, Result{}, err
		}
		if err := ValidateProvider(cfg); err != nil {
			return nil, Result{}, err
		}
		return cfg, Result{Path: path, Kind: KindProvider}, nil
	}

	if role, _ := probe["node_role"].(string); role == "protected_server" {
		var cfg AdvancedConfig
		if err := yaml.Unmarshal(raw, &cfg); err != nil {
			return nil, Result{}, err
		}
		if err := ValidateAdvanced(cfg); err != nil {
			return nil, Result{}, err
		}
		return cfg, Result{Path: path, Kind: KindAdvanced, Mode: cfg.Mode, Plan: cfg.DeploymentProfile}, nil
	}

	var cfg TenantConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, Result{}, err
	}
	if err := ValidateTenant(cfg); err != nil {
		return nil, Result{}, err
	}
	return cfg, Result{Path: path, Kind: KindTenant, Mode: cfg.Mode, Plan: cfg.Plan}, nil
}

func ValidateTenant(cfg TenantConfig) error {
	if !allowedModes[cfg.Mode] {
		return fmt.Errorf("invalid mode %q", cfg.Mode)
	}
	if strings.TrimSpace(cfg.Plan) == "" {
		return errors.New("plan is required")
	}
	if strings.TrimSpace(cfg.LicenseKey) == "" {
		return errors.New("license_key is required")
	}
	if len(cfg.Admin.AllowIPs) == 0 {
		return errors.New("admin.allow_ips must contain at least one CIDR")
	}
	for _, cidr := range cfg.Admin.AllowIPs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("invalid admin allow CIDR %q: %w", cidr, err)
		}
	}
	if strings.TrimSpace(cfg.Server.Interface) == "" {
		return errors.New("server.interface is required")
	}
	if cfg.Server.SSHPort <= 0 || cfg.Server.SSHPort > 65535 {
		return errors.New("server.ssh_port must be between 1 and 65535")
	}
	if !allowedProfiles[cfg.Protection.Profile] {
		return fmt.Errorf("invalid protection.profile %q", cfg.Protection.Profile)
	}
	if cfg.Mode == "full" || cfg.Website.Enabled {
		if err := validateTenantWebsite(cfg.Website); err != nil {
			return err
		}
	}
	return nil
}

func validateTenantWebsite(site TenantWebsite) error {
	if !allowedTLSModes[site.TLSMode] {
		return fmt.Errorf("invalid website.tls_mode %q", site.TLSMode)
	}
	if site.TLSMode == "full_tls" || site.TLSMode == "full_strict" {
		if strings.TrimSpace(site.CertFile) == "" || strings.TrimSpace(site.KeyFile) == "" {
			return fmt.Errorf("website.%s requires cert_file and key_file", site.TLSMode)
		}
	}
	if len(site.Sites) == 0 {
		return errors.New("website.sites must contain at least one site")
	}
	domains := map[string]int{}
	for i, s := range site.Sites {
		if len(s.Domains) == 0 {
			return fmt.Errorf("website.sites[%d].domains must not be empty", i)
		}
		for _, domain := range s.Domains {
			if prev, ok := domains[domain]; ok {
				return fmt.Errorf("duplicate domain %q in website.sites[%d] and website.sites[%d]", domain, prev, i)
			}
			domains[domain] = i
		}
		if err := validateBackendURL(s.Backend); err != nil {
			return fmt.Errorf("website.sites[%d].backend: %w", i, err)
		}
		for j, r := range s.Routes {
			if strings.TrimSpace(r.Path) == "" || !strings.HasPrefix(r.Path, "/") {
				return fmt.Errorf("website.sites[%d].routes[%d].path must start with /", i, j)
			}
			if err := validateBackendURL(r.Backend); err != nil {
				return fmt.Errorf("website.sites[%d].routes[%d].backend: %w", i, j, err)
			}
		}
	}
	return nil
}

func ValidateAdvanced(cfg AdvancedConfig) error {
	if cfg.NodeRole != "protected_server" {
		return fmt.Errorf("advanced node_role must be protected_server")
	}
	if !allowedModes[cfg.Mode] {
		return fmt.Errorf("invalid mode %q", cfg.Mode)
	}
	if strings.TrimSpace(cfg.DeploymentProfile) == "" {
		return errors.New("deployment_profile is required")
	}
	if cfg.License.RequireValidLicense {
		if strings.TrimSpace(cfg.License.File) == "" {
			return errors.New("license.file is required when require_valid_license is true")
		}
		if strings.TrimSpace(cfg.License.ProviderPublicKey) == "" {
			return errors.New("license.provider_public_key is required when require_valid_license is true")
		}
		if strings.TrimSpace(cfg.ServerIdentity.FingerprintSaltID) == "" {
			return errors.New("server_identity.fingerprint_salt_id is required when license is enforced")
		}
	}
	if cfg.Safety.RollbackTimerSeconds < 0 {
		return errors.New("safety.rollback_timer_seconds must not be negative")
	}
	if err := validateResourceGovernor(cfg.ResourceGovernor); err != nil {
		return err
	}
	if err := validateAdvancedXDP(cfg.ServerProtection.XDP); err != nil {
		return err
	}
	if err := validateServerDDOS(cfg.ServerProtection.DDOS); err != nil {
		return err
	}
	if err := validateWebsiteDefense(cfg.WebsiteProtection); err != nil {
		return err
	}
	if err := validateAdvancedUpdates(cfg.Updates); err != nil {
		return err
	}
	if err := validateAdvancedRuntimeSecurity(cfg.RuntimeSecurity); err != nil {
		return err
	}
	if err := validateAdvancedTelemetry(cfg.Telemetry); err != nil {
		return err
	}
	for _, port := range cfg.ServerProtection.Nftables.AllowPorts {
		if port <= 0 || port > 65535 {
			return fmt.Errorf("server_protection.nftables.allow_ports contains invalid port %d", port)
		}
	}
	for _, cidr := range cfg.ServerProtection.Nftables.AdminIPs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("invalid server_protection.nftables.admin_ips CIDR %q: %w", cidr, err)
		}
	}
	if len(cfg.BackendPools) == 0 {
		return errors.New("backend_pools must not be empty")
	}
	pools := map[string]bool{}
	for _, pool := range cfg.BackendPools {
		if strings.TrimSpace(pool.ID) == "" {
			return errors.New("backend pool id is required")
		}
		if pools[pool.ID] {
			return fmt.Errorf("duplicate backend pool %q", pool.ID)
		}
		pools[pool.ID] = true
		if len(pool.Upstreams) == 0 {
			return fmt.Errorf("backend pool %q must have upstreams", pool.ID)
		}
		for _, upstream := range pool.Upstreams {
			if err := validateBackendURL(upstream.URL); err != nil {
				return fmt.Errorf("backend pool %q upstream %q: %w", pool.ID, upstream.ID, err)
			}
		}
	}
	if cfg.Mode == "full" && len(cfg.Sites) == 0 {
		return errors.New("sites must not be empty in full mode")
	}
	domains := map[string]string{}
	for _, site := range cfg.Sites {
		if strings.TrimSpace(site.ID) == "" {
			return errors.New("site id is required")
		}
		if site.TLS.OriginMode != "" {
			if !allowedTLSModes[site.TLS.OriginMode] {
				return fmt.Errorf("site %q has invalid tls.origin_mode %q", site.ID, site.TLS.OriginMode)
			}
			if site.TLS.OriginMode == "full_tls" || site.TLS.OriginMode == "full_strict" {
				if strings.TrimSpace(site.TLS.CertificateFile) == "" || strings.TrimSpace(site.TLS.PrivateKeyFile) == "" {
					return fmt.Errorf("site %q tls.%s requires certificate_file and private_key_file", site.ID, site.TLS.OriginMode)
				}
			}
		}
		if len(site.Domains) == 0 {
			return fmt.Errorf("site %q must have domains", site.ID)
		}
		for _, domain := range site.Domains {
			if prev, ok := domains[domain]; ok {
				return fmt.Errorf("duplicate domain %q in sites %q and %q", domain, prev, site.ID)
			}
			domains[domain] = site.ID
		}
		if !pools[site.DefaultBackendPool] {
			return fmt.Errorf("site %q references unknown default backend pool %q", site.ID, site.DefaultBackendPool)
		}
		for _, route := range site.Routes {
			if strings.TrimSpace(route.Path) == "" || !strings.HasPrefix(route.Path, "/") {
				return fmt.Errorf("site %q route path must start with /", site.ID)
			}
			if route.BackendPool != "" && !pools[route.BackendPool] {
				return fmt.Errorf("site %q route %q references unknown backend pool %q", site.ID, route.Path, route.BackendPool)
			}
		}
	}
	return nil
}

func ValidateProvider(cfg ProviderConfig) error {
	if cfg.Provider.NodeRole != "provider_license_server" {
		return errors.New("provider.node_role must be provider_license_server")
	}
	if strings.TrimSpace(cfg.Provider.Name) == "" {
		return errors.New("provider.name is required")
	}
	if strings.TrimSpace(cfg.Provider.SigningKeyFile) == "" {
		return errors.New("provider.signing_key_file is required")
	}
	if strings.TrimSpace(cfg.Provider.PublicKeyFile) == "" {
		return errors.New("provider.public_key_file is required")
	}
	if cfg.Storage.Driver != "file" {
		return errors.New("storage.driver must be file in MVP")
	}
	if strings.TrimSpace(cfg.Storage.RootDir) == "" {
		return errors.New("storage.root_dir is required")
	}
	if cfg.Licenses.DefaultGraceDays < 0 {
		return errors.New("licenses.default_grace_days must not be negative")
	}
	if len(cfg.Licenses.Plans) == 0 {
		return errors.New("licenses.plans must not be empty")
	}
	for name, plan := range cfg.Licenses.Plans {
		if strings.TrimSpace(name) == "" {
			return errors.New("licenses.plans contains empty plan name")
		}
		if len(plan.AllowedModes) == 0 {
			return fmt.Errorf("licenses.plans.%s.allowed_modes must not be empty", name)
		}
		for _, mode := range plan.AllowedModes {
			if !allowedModes[mode] {
				return fmt.Errorf("licenses.plans.%s has invalid mode %q", name, mode)
			}
		}
		if len(plan.Features) == 0 {
			return fmt.Errorf("licenses.plans.%s.features must not be empty", name)
		}
	}
	if cfg.Updates.RollbackRetention < 0 {
		return errors.New("updates.rollback_retention must not be negative")
	}
	for _, channel := range cfg.Updates.Channels {
		channel = strings.TrimSpace(channel)
		if channel == "" {
			return errors.New("updates.channels contains empty channel")
		}
	}
	return nil
}

func validateBackendURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return errors.New("backend URL is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("backend URL must use http or https")
	}
	if u.Host == "" {
		return errors.New("backend URL host is required")
	}
	return nil
}

func validateResourceGovernor(cfg ResourceGovernorConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.Baseline.LearningDays < 0 {
		return errors.New("resource_governor.baseline.learning_days must not be negative")
	}
	if cfg.Baseline.MinSamples < 0 {
		return errors.New("resource_governor.baseline.min_samples must not be negative")
	}
	if cfg.Hysteresis.MinLevelHoldSeconds < 0 {
		return errors.New("resource_governor.hysteresis.min_level_hold_seconds must not be negative")
	}
	if cfg.Hysteresis.CooldownSeconds < 0 {
		return errors.New("resource_governor.hysteresis.cooldown_seconds must not be negative")
	}
	if cfg.Hysteresis.RequireRecoverySamples < 0 {
		return errors.New("resource_governor.hysteresis.require_recovery_samples must not be negative")
	}
	if err := validateGovernorThreshold("elevated", cfg.Levels.Elevated); err != nil {
		return err
	}
	if err := validateGovernorThreshold("attack", cfg.Levels.Attack); err != nil {
		return err
	}
	if err := validateGovernorThreshold("lockdown", cfg.Levels.Lockdown); err != nil {
		return err
	}
	return nil
}

func validateWebsiteDefense(cfg WebsiteProtection) error {
	if cfg.WAF.Enabled {
		if strings.TrimSpace(cfg.WAF.Engine) == "" {
			return errors.New("website_protection.waf.engine is required when WAF is enabled")
		}
		if !allowedWAFEngines[cfg.WAF.Engine] {
			return fmt.Errorf("website_protection.waf.engine must be coraza or modsecurity, got %q", cfg.WAF.Engine)
		}
		if cfg.WAF.AnomalyThreshold < 0 {
			return errors.New("website_protection.waf.anomaly_threshold must not be negative")
		}
	}
	if cfg.Bot.ScoreChallenge < 0 || cfg.Bot.ScoreChallenge > 100 {
		return errors.New("website_protection.bot.score_challenge must be between 0 and 100")
	}
	if cfg.Bot.ScoreBlock < 0 || cfg.Bot.ScoreBlock > 100 {
		return errors.New("website_protection.bot.score_block must be between 0 and 100")
	}
	if cfg.Bot.Enabled && cfg.Bot.ScoreBlock > 0 && cfg.Bot.ScoreChallenge > cfg.Bot.ScoreBlock {
		return errors.New("website_protection.bot.score_challenge must be less than or equal to score_block")
	}
	return nil
}

func validateAdvancedXDP(cfg AdvancedXDP) error {
	if !cfg.Enabled {
		return nil
	}
	switch strings.TrimSpace(cfg.Mode) {
	case "", "generic", "native", "offload":
	default:
		return fmt.Errorf("server_protection.xdp.mode must be generic, native, or offload, got %q", cfg.Mode)
	}
	if strings.TrimSpace(cfg.Section) != "" && strings.ContainsAny(cfg.Section, " \t\n\r/") {
		return errors.New("server_protection.xdp.section must be a simple ELF section name")
	}
	for _, path := range []struct {
		name  string
		value string
	}{
		{"program_path", cfg.ProgramPath},
		{"allowlist_file", cfg.AllowlistFile},
		{"blocklist_file", cfg.BlocklistFile},
	} {
		if strings.Contains(path.value, "\x00") {
			return fmt.Errorf("server_protection.xdp.%s contains NUL byte", path.name)
		}
	}
	return nil
}

func validateServerDDOS(cfg ServerDDOS) error {
	if cfg.PerIPPPS < 0 ||
		cfg.PerSubnet24PPS < 0 ||
		cfg.SynPerIPPerSecond < 0 ||
		cfg.UDPPerIPPerSecond < 0 ||
		cfg.ICMPPerIPPerSecond < 0 ||
		cfg.TemporaryBlockSeconds < 0 ||
		cfg.GreylistSeconds < 0 {
		return errors.New("server_protection.ddos thresholds must not be negative")
	}
	return nil
}

func validateAdvancedUpdates(cfg AdvancedUpdates) error {
	if !cfg.Enabled {
		return nil
	}
	if strings.TrimSpace(cfg.Channel) == "" {
		return errors.New("updates.channel is required when updates are enabled")
	}
	if strings.TrimSpace(cfg.ManifestURL) == "" {
		return errors.New("updates.manifest_url is required when updates are enabled")
	}
	return nil
}

func validateAdvancedRuntimeSecurity(cfg AdvancedRuntimeSecurity) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.FileIntegrity.Enabled && len(cfg.FileIntegrity.Paths) == 0 {
		return errors.New("runtime_security.file_integrity.paths must not be empty when file integrity is enabled")
	}
	for _, path := range cfg.FileIntegrity.Paths {
		if strings.TrimSpace(path) == "" {
			return errors.New("runtime_security.file_integrity.paths contains empty path")
		}
	}
	for _, name := range cfg.AlertWhenWebUserExecutes {
		if strings.TrimSpace(name) == "" {
			return errors.New("runtime_security.alert_when_web_user_executes contains empty process name")
		}
	}
	return nil
}

func validateAdvancedTelemetry(cfg AdvancedTelemetry) error {
	if cfg.HealthReport.SendIntervalSeconds < 0 {
		return errors.New("telemetry.health_report.send_interval_seconds must not be negative")
	}
	return nil
}

func validateGovernorThreshold(name string, threshold ResourceGovernorLevelThreshold) error {
	if err := validatePercent("cpu_percent", threshold.CPUPercent); err != nil {
		return fmt.Errorf("resource_governor.levels.%s.%w", name, err)
	}
	if err := validatePercent("ram_available_percent", threshold.RAMAvailablePercent); err != nil {
		return fmt.Errorf("resource_governor.levels.%s.%w", name, err)
	}
	if threshold.Load1 < 0 {
		return fmt.Errorf("resource_governor.levels.%s.load1 must not be negative", name)
	}
	if err := validatePercent("conntrack_percent", threshold.ConntrackPercent); err != nil {
		return fmt.Errorf("resource_governor.levels.%s.%w", name, err)
	}
	if threshold.BackendLatencyMS < 0 {
		return fmt.Errorf("resource_governor.levels.%s.backend_latency_ms must not be negative", name)
	}
	return nil
}

func validatePercent(name string, value float64) error {
	if value < 0 || value > 100 {
		return fmt.Errorf("%s must be between 0 and 100", name)
	}
	return nil
}
