package client

import "kiro_waf/internal/shared/config"

// ClientYAMLConfig extends the tenant YAML schema with WAF-specific runtime fields.
// It embeds the shared tenant fields (mode, plan, license_key, admin, website, protection)
// and adds a `client` top-level section for WAF runtime settings.
type ClientYAMLConfig struct {
	Mode       string                 `yaml:"mode"`
	Plan       string                 `yaml:"plan"`
	LicenseKey string                 `yaml:"license_key"`
	Admin      config.TenantAdmin     `yaml:"admin"`
	Server     config.TenantServer    `yaml:"server"`
	Website    config.TenantWebsite   `yaml:"website"`
	Protection config.TenantProtection `yaml:"protection"`

	// WAF-specific runtime settings
	Client ClientSection `yaml:"client"`
}

// ClientSection contains WAF runtime settings not present in the tenant config schema.
type ClientSection struct {
	CookieSecret     string `yaml:"cookie_secret"`
	MasterURL        string `yaml:"master_url"`
	ListenAddr       string `yaml:"listen_addr"`
	NodeID           string `yaml:"node_id"`
	PoWDifficulty    int    `yaml:"pow_difficulty"`
	HoldSeconds      int    `yaml:"hold_seconds"`
	RPMPerIP         int    `yaml:"rpm_per_ip"`
	SubnetRPM        int    `yaml:"subnet_rpm"`
	HardBlockAfter   int    `yaml:"hard_block_after"`
	BlockTTLSeconds  int    `yaml:"block_ttl_seconds"`
	BlocklistFile    string `yaml:"blocklist_file"`
	XDPSyncCommand   string `yaml:"xdp_sync_command"`
	HeartbeatSeconds int    `yaml:"heartbeat_seconds"`
	UpdateSeconds    int    `yaml:"update_seconds"`
	ChallengeAllNew  bool   `yaml:"challenge_all_new"`

	// Transparent challenge / escalation
	TransparentTTL      int    `yaml:"transparent_ttl"`
	CookieShortTTL      int    `yaml:"cookie_short_ttl"`
	EscalationThreshold int    `yaml:"escalation_threshold"`
	EscalationCooldown  int    `yaml:"escalation_cooldown"`
	CookieRateLimit     int    `yaml:"cookie_rate_limit"`
	CFTrustMode         string `yaml:"cf_trust_mode"`
	XDPBlockedCountries string `yaml:"xdp_blocked_countries"`
	GeoIPCSVPath        string `yaml:"geoip_csv_path"`
}
