package client

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// protectionProfileDefaults maps protection.profile values to rate-limit defaults.
var protectionProfileDefaults = map[string]struct {
	RPMPerIP       int
	SubnetRPM      int
	HardBlockAfter int
}{
	"light":    {RPMPerIP: 200, SubnetRPM: 3000, HardBlockAfter: 600},
	"balanced": {RPMPerIP: 120, SubnetRPM: 1800, HardBlockAfter: 360},
	"strict":   {RPMPerIP: 60, SubnetRPM: 900, HardBlockAfter: 180},
	"lockdown": {RPMPerIP: 30, SubnetRPM: 450, HardBlockAfter: 90},
}

// LoadClientConfig loads configuration from YAML file with env var overrides.
// It orchestrates the full loading pipeline: YAML → env overrides → defaults → validate.
// Returns a fully populated clientConfig or an error describing all validation failures.
func LoadClientConfig(yamlPath string) (clientConfig, error) {
	var cfg clientConfig
	var loadedFromYAML bool

	// Try to load from YAML file
	if _, err := os.Stat(yamlPath); err == nil {
		// YAML file exists — parse it
		parsed, parseErr := loadFromYAML(yamlPath)
		if parseErr != nil {
			return clientConfig{}, fmt.Errorf("failed to parse config file %s: %w", yamlPath, parseErr)
		}
		cfg = parsed
		loadedFromYAML = true
	} else if errors.Is(err, os.ErrNotExist) {
		// YAML file doesn't exist — check if env vars are available (legacy mode)
		if hasLegacyEnvVars() {
			log.Printf("WARNING: YAML config file %s not found, loading from environment variables (legacy mode). Consider migrating to YAML.", yamlPath)
		} else {
			// Neither YAML nor env vars exist — fatal error
			return clientConfig{}, fmt.Errorf(
				"no configuration source found: YAML file %s does not exist and no environment variables (KIRO_LICENSE_KEY, KIRO_BACKEND_URL, KIRO_MASTER_URL) are set",
				yamlPath,
			)
		}
	} else {
		// Some other error accessing the file (permission denied, etc.)
		return clientConfig{}, fmt.Errorf("cannot access config file %s: %w", yamlPath, err)
	}

	_ = loadedFromYAML // used for logging context if needed

	// Apply environment variable overrides
	applyEnvOverrides(&cfg)

	// Apply built-in defaults for remaining zero-value fields
	applyDefaults(&cfg)

	// Validate the final config
	if err := validateClientConfig(cfg); err != nil {
		return clientConfig{}, err
	}

	return cfg, nil
}

// loadFromYAML reads a YAML file, unmarshals it into ClientYAMLConfig,
// and maps fields to clientConfig using the documented mapping table.
func loadFromYAML(path string) (clientConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return clientConfig{}, fmt.Errorf("read file: %w", err)
	}

	var yamlCfg ClientYAMLConfig
	if err := yaml.Unmarshal(data, &yamlCfg); err != nil {
		return clientConfig{}, fmt.Errorf("parse YAML: %w", err)
	}

	cfg := clientConfig{}

	// Map top-level fields
	cfg.LicenseKey = yamlCfg.LicenseKey

	// Map website.sites[0].backend → BackendURL
	if len(yamlCfg.Website.Sites) > 0 {
		cfg.BackendURL = yamlCfg.Website.Sites[0].Backend
	}

	// Map admin.allow_ips → AdminIPs
	if len(yamlCfg.Admin.AllowIPs) > 0 {
		cfg.AdminIPs = make([]string, len(yamlCfg.Admin.AllowIPs))
		copy(cfg.AdminIPs, yamlCfg.Admin.AllowIPs)
	}

	// Map protection.profile → rate-limit defaults
	if defaults, ok := protectionProfileDefaults[yamlCfg.Protection.Profile]; ok {
		cfg.RPMPerIP = defaults.RPMPerIP
		cfg.SubnetRPM = defaults.SubnetRPM
		cfg.HardBlockAfter = defaults.HardBlockAfter
	} else if yamlCfg.Protection.Profile != "" {
		// Unrecognized profile — log warning, use balanced defaults
		log.Printf("WARNING: unrecognized protection.profile %q, using 'balanced' defaults", yamlCfg.Protection.Profile)
		cfg.RPMPerIP = protectionProfileDefaults["balanced"].RPMPerIP
		cfg.SubnetRPM = protectionProfileDefaults["balanced"].SubnetRPM
		cfg.HardBlockAfter = protectionProfileDefaults["balanced"].HardBlockAfter
	}

	// Map client section fields
	cfg.CookieSecret = yamlCfg.Client.CookieSecret
	cfg.MasterURL = yamlCfg.Client.MasterURL
	cfg.ListenAddr = yamlCfg.Client.ListenAddr
	cfg.NodeID = yamlCfg.Client.NodeID
	cfg.PoWDifficulty = yamlCfg.Client.PoWDifficulty
	cfg.HoldSeconds = yamlCfg.Client.HoldSeconds
	cfg.BlockTTLSeconds = yamlCfg.Client.BlockTTLSeconds
	cfg.BlocklistFile = yamlCfg.Client.BlocklistFile
	cfg.XDPSyncCommand = yamlCfg.Client.XDPSyncCommand
	cfg.HeartbeatSeconds = yamlCfg.Client.HeartbeatSeconds
	cfg.UpdateSeconds = yamlCfg.Client.UpdateSeconds
	cfg.ChallengeAllNew = yamlCfg.Client.ChallengeAllNew

	// Transparent challenge / escalation fields
	cfg.TransparentTTL = yamlCfg.Client.TransparentTTL
	cfg.CookieShortTTL = yamlCfg.Client.CookieShortTTL
	cfg.EscalationThreshold = yamlCfg.Client.EscalationThreshold
	cfg.EscalationCooldown = yamlCfg.Client.EscalationCooldown
	cfg.CookieRateLimit = yamlCfg.Client.CookieRateLimit
	cfg.CFTrustMode = yamlCfg.Client.CFTrustMode
	cfg.XDPBlockedCountries = yamlCfg.Client.XDPBlockedCountries
	cfg.GeoIPCSVPath = yamlCfg.Client.GeoIPCSVPath

	// If client section explicitly sets rate-limit fields, they override profile defaults
	if yamlCfg.Client.RPMPerIP != 0 {
		cfg.RPMPerIP = yamlCfg.Client.RPMPerIP
	}
	if yamlCfg.Client.SubnetRPM != 0 {
		cfg.SubnetRPM = yamlCfg.Client.SubnetRPM
	}
	if yamlCfg.Client.HardBlockAfter != 0 {
		cfg.HardBlockAfter = yamlCfg.Client.HardBlockAfter
	}

	return cfg, nil
}

// applyEnvOverrides overlays non-empty environment variables onto the config.
// For each field, if the corresponding env var is set and non-empty, it overrides the config value.
func applyEnvOverrides(cfg *clientConfig) {
	if v := envTrimmed("KIRO_LICENSE_KEY"); v != "" {
		cfg.LicenseKey = v
	}
	if v := envTrimmed("KIRO_BACKEND_URL"); v != "" {
		cfg.BackendURL = v
	}
	if v := envTrimmed("KIRO_MASTER_URL"); v != "" {
		cfg.MasterURL = v
	}
	if v := envTrimmed("KIRO_CLIENT_COOKIE_SECRET"); v != "" {
		cfg.CookieSecret = v
	}
	if v := envTrimmed("KIRO_CLIENT_LISTEN"); v != "" {
		cfg.ListenAddr = v
	}
	if v := envTrimmed("KIRO_NODE_ID"); v != "" {
		cfg.NodeID = v
	}
	if v := envTrimmed("KIRO_ADMIN_IPS"); v != "" {
		var ips []string
		for _, ip := range strings.Split(v, ",") {
			trimmed := strings.TrimSpace(ip)
			if trimmed != "" {
				ips = append(ips, trimmed)
			}
		}
		cfg.AdminIPs = ips
	}
	if v := envTrimmedInt("KIRO_RPM_PER_IP"); v > 0 {
		cfg.RPMPerIP = v
	}
	if v := envTrimmedInt("KIRO_SUBNET_RPM"); v > 0 {
		cfg.SubnetRPM = v
	}
	if v := envTrimmedInt("KIRO_HARD_BLOCK_AFTER"); v > 0 {
		cfg.HardBlockAfter = v
	}
	if v := envTrimmedInt("KIRO_POW_DIFFICULTY"); v > 0 {
		cfg.PoWDifficulty = v
	}
	if v := envTrimmedInt("KIRO_HOLD_SECONDS"); v > 0 {
		cfg.HoldSeconds = v
	}
	if v := envTrimmedInt("KIRO_BLOCK_TTL_SECONDS"); v > 0 {
		cfg.BlockTTLSeconds = v
	}
	if v := envTrimmed("KIRO_XDP_BLOCKLIST_FILE"); v != "" {
		cfg.BlocklistFile = v
	}
	if v := envTrimmed("KIRO_XDP_SYNC_COMMAND"); v != "" {
		cfg.XDPSyncCommand = v
	}
	if v := envTrimmedInt("KIRO_HEARTBEAT_SECONDS"); v > 0 {
		cfg.HeartbeatSeconds = v
	}
	if v := envTrimmedInt("KIRO_UPDATE_SECONDS"); v > 0 {
		cfg.UpdateSeconds = v
	}
	if v := envTrimmed("KIRO_CHALLENGE_ALL_NEW"); v != "" {
		cfg.ChallengeAllNew = v == "true" || v == "1"
	}
	if v := envTrimmedInt("KIRO_TRANSPARENT_TTL"); v > 0 {
		cfg.TransparentTTL = v
	}
	if v := envTrimmedInt("KIRO_COOKIE_SHORT_TTL"); v > 0 {
		cfg.CookieShortTTL = v
	}
	if v := envTrimmedInt("KIRO_ESCALATION_THRESHOLD"); v > 0 {
		cfg.EscalationThreshold = v
	}
	if v := envTrimmedInt("KIRO_ESCALATION_COOLDOWN"); v > 0 {
		cfg.EscalationCooldown = v
	}
	if v := envTrimmedInt("KIRO_COOKIE_RATE_LIMIT"); v > 0 {
		cfg.CookieRateLimit = v
	}
	if v := envTrimmed("KIRO_CF_TRUST_MODE"); v != "" {
		cfg.CFTrustMode = v
	}
	if v := envTrimmed("KIRO_XDP_BLOCKED_COUNTRIES"); v != "" {
		cfg.XDPBlockedCountries = v
	}
	if v := envTrimmed("KIRO_GEOIP_CSV_PATH"); v != "" {
		cfg.GeoIPCSVPath = v
	}
}

// applyDefaults fills zero-value fields with built-in defaults.
func applyDefaults(cfg *clientConfig) {
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8090"
	}
	if cfg.NodeID == "" {
		cfg.NodeID = getHostname()
	}
	if cfg.PoWDifficulty == 0 {
		cfg.PoWDifficulty = 4
	}
	if cfg.HoldSeconds == 0 {
		cfg.HoldSeconds = 2
	}
	if cfg.BlockTTLSeconds == 0 {
		cfg.BlockTTLSeconds = 900
	}
	if cfg.BlocklistFile == "" {
		cfg.BlocklistFile = "/var/lib/kiro/xdp-blocklist.txt"
	}
	if cfg.XDPSyncCommand == "" {
		cfg.XDPSyncCommand = "/usr/local/bin/kiro-xdp-sync"
	}
	if cfg.HeartbeatSeconds == 0 {
		cfg.HeartbeatSeconds = 60
	}
	if cfg.UpdateSeconds == 0 {
		cfg.UpdateSeconds = 300
	}
	if cfg.TransparentTTL == 0 {
		cfg.TransparentTTL = 30
	}
	if cfg.CookieShortTTL == 0 {
		cfg.CookieShortTTL = 300
	}
	if cfg.EscalationThreshold == 0 {
		cfg.EscalationThreshold = 3
	}
	if cfg.EscalationCooldown == 0 {
		cfg.EscalationCooldown = 600
	}
	if cfg.CookieRateLimit == 0 {
		cfg.CookieRateLimit = 300
	}
	if cfg.CFTrustMode == "" {
		cfg.CFTrustMode = "strict"
	}

	// Rate-limit defaults: if still zero after YAML + env, use balanced profile
	if cfg.RPMPerIP == 0 {
		cfg.RPMPerIP = protectionProfileDefaults["balanced"].RPMPerIP
	}
	if cfg.SubnetRPM == 0 {
		cfg.SubnetRPM = protectionProfileDefaults["balanced"].SubnetRPM
	}
	if cfg.HardBlockAfter == 0 {
		cfg.HardBlockAfter = protectionProfileDefaults["balanced"].HardBlockAfter
	}

	// AdminIPs defaults to empty slice (not nil) for consistency
	if cfg.AdminIPs == nil {
		cfg.AdminIPs = []string{}
	}
}

// hasLegacyEnvVars checks if any of the key environment variables are set,
// indicating the user intends to configure via env vars (legacy mode).
func hasLegacyEnvVars() bool {
	keys := []string{
		"KIRO_LICENSE_KEY",
		"KIRO_BACKEND_URL",
		"KIRO_MASTER_URL",
		"KIRO_CLIENT_COOKIE_SECRET",
	}
	for _, key := range keys {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return true
		}
	}
	return false
}

// envTrimmed returns the trimmed value of an environment variable, or empty string if unset/empty.
func envTrimmed(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

// envTrimmedInt returns the integer value of an environment variable, or 0 if unset/empty/invalid.
func envTrimmedInt(key string) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}
