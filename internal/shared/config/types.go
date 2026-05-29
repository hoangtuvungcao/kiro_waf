package config

type Kind string

const (
	KindTenant   Kind = "tenant"
	KindAdvanced Kind = "advanced"
	KindProvider Kind = "provider"
)

type Result struct {
	Path string
	Kind Kind
	Mode string
	Plan string
}

type RuntimeConfig struct {
	SourceKind       Kind
	Mode             string
	Plan             string
	Paths            RuntimePaths
	Safety           RuntimeSafety
	License          RuntimeLicense
	Identity         RuntimeIdentity
	Firewall         RuntimeFirewall
	XDP              RuntimeXDP
	CFOriginLock     RuntimeCloudflareOriginLock
	AdminCIDRs       []string
	Interface        string
	SSHPort          int
	Cloudflare       bool
	TLSMode          string
	BackendPools     []RuntimeBackendPool
	Sites            []RuntimeSite
	Protection       RuntimeProtection
	WAF              RuntimeWAF
	Bot              RuntimeBot
	ResourceGovernor ResourceGovernorConfig
	Updates          RuntimeUpdates
	RuntimeSecurity  RuntimeSecurity
	Telemetry        RuntimeTelemetry
}

type RuntimePaths struct {
	StateDir          string
	LastGoodConfigDir string
}

type RuntimeSafety struct {
	DryRunBeforeApply                 bool
	RequireAdminIPBeforeFirewallApply bool
	RollbackTimerSeconds              int
	RequireLocalConsoleWarning        bool
}

type RuntimeFirewall struct {
	Enabled                 bool
	ProtectConntrack        bool
	AllowPorts              []int
	SSHAdminOnly            bool
	AdminCIDRs              []string
	TemporaryBlockSeconds   int
	RequireAdminBeforeApply bool
}

type RuntimeXDP struct {
	Enabled             bool
	Mode                string
	ProgramPath         string
	Section             string
	DropPrivateSourceIP bool
	DropMalformed       bool
	DropFragments       bool
	RateLimitEnabled    bool
	WindowSeconds       int
	PerIPPPS            int
	PerSubnet24PPS      int
	SynPPS              int
	UDPPPS              int
	ICMPPPS             int
	AllowlistFile       string
	BlocklistFile       string
}

type RuntimeCloudflareOriginLock struct {
	Enabled               bool
	RequireProxiedTraffic bool
	BlockDirectOriginHTTP bool
	IPv4File              string
	IPv6File              string
}

type RuntimeLicense struct {
	File                string
	ProviderPublicKey   string
	RequireValidLicense bool
	AllowGracePeriod    bool
}

type RuntimeIdentity struct {
	UseMachineID      bool
	UsePrimaryMAC     bool
	UseAllMACsHash    bool
	FingerprintSaltID string
}

type RuntimeBackendPool struct {
	ID        string
	Upstreams []RuntimeUpstream
}

type RuntimeUpstream struct {
	ID  string
	URL string
}

type RuntimeSite struct {
	ID                 string
	Domains            []string
	TLSMode            string
	CertFile           string
	KeyFile            string
	DefaultBackendPool string
	Routes             []RuntimeRoute
}

type RuntimeRoute struct {
	Path              string
	BackendPool       string
	Protection        string
	RPMPerIP          int
	CacheSeconds      int
	MaxBodyMB         int
	WAFExcludeRuleIDs []string
}

type RuntimeProtection struct {
	Profile        string
	WAF            bool
	Bot            bool
	AutoAttackMode bool
}

type RuntimeWAF struct {
	Enabled          bool
	Engine           string
	OWASPCRS         bool
	AnomalyThreshold int
}

type RuntimeBot struct {
	Enabled             bool
	CookieChallenge     bool
	JSChallenge         bool
	ProofOfWork         bool
	ScoreChallenge      int
	ScoreBlock          int
	ChallengeCookieName string
	TrustedClientCIDRs  []string
}

type RuntimeUpdates struct {
	Enabled                     bool
	Channel                     string
	ManifestURL                 string
	RequireSignedManifest       bool
	AutoRollbackOnHealthFailure bool
}

type RuntimeSecurity struct {
	Enabled              bool
	AuditProcessExec     bool
	FileIntegrityEnabled bool
	FileIntegrityPaths   []string
	AlertProcessNames    []string
	WebUsers             []string
}

type RuntimeTelemetry struct {
	Enabled                   bool
	HealthReportEnabled       bool
	HealthSendIntervalSeconds int
	Privacy                   RuntimePrivacy
}

type RuntimePrivacy struct {
	SendRequestBody         bool
	SendCookie              bool
	SendAuthorizationHeader bool
	SendRawClientIP         bool
	HashClientIP            bool
	RedactSecrets           bool
}

type ResourceGovernorConfig struct {
	Enabled    bool                       `yaml:"enabled" json:"enabled"`
	Baseline   ResourceGovernorBaseline   `yaml:"baseline" json:"baseline"`
	Hysteresis ResourceGovernorHysteresis `yaml:"hysteresis" json:"hysteresis"`
	Levels     ResourceGovernorLevels     `yaml:"levels" json:"levels"`
	Actions    ResourceGovernorActions    `yaml:"actions" json:"actions"`
}

type ResourceGovernorBaseline struct {
	Enabled      bool   `yaml:"enabled" json:"enabled"`
	LearningDays int    `yaml:"learning_days" json:"learning_days"`
	MinSamples   int    `yaml:"min_samples" json:"min_samples"`
	StoreFile    string `yaml:"store_file" json:"store_file"`
}

type ResourceGovernorHysteresis struct {
	Enabled                bool `yaml:"enabled" json:"enabled"`
	MinLevelHoldSeconds    int  `yaml:"min_level_hold_seconds" json:"min_level_hold_seconds"`
	CooldownSeconds        int  `yaml:"cooldown_seconds" json:"cooldown_seconds"`
	RequireRecoverySamples int  `yaml:"require_recovery_samples" json:"require_recovery_samples"`
}

type ResourceGovernorLevels struct {
	Elevated ResourceGovernorLevelThreshold `yaml:"elevated" json:"elevated"`
	Attack   ResourceGovernorLevelThreshold `yaml:"attack" json:"attack"`
	Lockdown ResourceGovernorLevelThreshold `yaml:"lockdown" json:"lockdown"`
}

type ResourceGovernorLevelThreshold struct {
	CPUPercent          float64 `yaml:"cpu_percent" json:"cpu_percent"`
	RAMAvailablePercent float64 `yaml:"ram_available_percent" json:"ram_available_percent"`
	Load1               float64 `yaml:"load1" json:"load1"`
	ConntrackPercent    float64 `yaml:"conntrack_percent" json:"conntrack_percent"`
	BackendLatencyMS    float64 `yaml:"backend_latency_ms" json:"backend_latency_ms"`
}

type ResourceGovernorActions struct {
	Elevated ResourceGovernorElevatedActions `yaml:"elevated" json:"elevated"`
	Attack   ResourceGovernorAttackActions   `yaml:"attack" json:"attack"`
	Lockdown ResourceGovernorLockdownActions `yaml:"lockdown" json:"lockdown"`
}

type ResourceGovernorElevatedActions struct {
	TightenRateLimits            bool `yaml:"tighten_rate_limits" json:"tighten_rate_limits"`
	EnableChallengeForNewClients bool `yaml:"enable_challenge_for_new_clients" json:"enable_challenge_for_new_clients"`
	IncreaseCache                bool `yaml:"increase_cache" json:"increase_cache"`
}

type ResourceGovernorAttackActions struct {
	TemporaryBlockBadClients bool `yaml:"temporary_block_bad_clients" json:"temporary_block_bad_clients"`
	DisableExpensiveRoutes   bool `yaml:"disable_expensive_routes" json:"disable_expensive_routes"`
	LowerTimeouts            bool `yaml:"lower_timeouts" json:"lower_timeouts"`
}

type ResourceGovernorLockdownActions struct {
	AllowAdminAndKnownClientsOnly bool `yaml:"allow_admin_and_known_clients_only" json:"allow_admin_and_known_clients_only"`
	ProtectBackendFirst           bool `yaml:"protect_backend_first" json:"protect_backend_first"`
}

type TenantConfig struct {
	Mode       string           `yaml:"mode"`
	Plan       string           `yaml:"plan"`
	LicenseKey string           `yaml:"license_key"`
	Admin      TenantAdmin      `yaml:"admin"`
	Server     TenantServer     `yaml:"server"`
	Website    TenantWebsite    `yaml:"website"`
	Protection TenantProtection `yaml:"protection"`
	Updates    TenantUpdates    `yaml:"updates"`
	Telemetry  TenantTelemetry  `yaml:"telemetry"`
}

type TenantAdmin struct {
	AllowIPs []string `yaml:"allow_ips"`
}

type TenantServer struct {
	Interface  string `yaml:"interface"`
	SSHPort    int    `yaml:"ssh_port"`
	AllowPorts []int  `yaml:"allow_ports"`
}

type TenantWebsite struct {
	Enabled    bool         `yaml:"enabled"`
	Cloudflare bool         `yaml:"cloudflare"`
	TLSMode    string       `yaml:"tls_mode"`
	CertFile   string       `yaml:"cert_file"`
	KeyFile    string       `yaml:"key_file"`
	Sites      []TenantSite `yaml:"sites"`
}

type TenantSite struct {
	Domains []string      `yaml:"domains"`
	Backend string        `yaml:"backend"`
	Routes  []TenantRoute `yaml:"routes"`
}

type TenantRoute struct {
	Path       string `yaml:"path"`
	Backend    string `yaml:"backend"`
	Protection string `yaml:"protection"`
}

type TenantProtection struct {
	Profile        string `yaml:"profile"`
	WAF            bool   `yaml:"waf"`
	Bot            bool   `yaml:"bot"`
	AutoAttackMode bool   `yaml:"auto_attack_mode"`
}

type TenantUpdates struct {
	AutoSecurityUpdates bool `yaml:"auto_security_updates"`
}

type TenantTelemetry struct {
	Enabled bool `yaml:"enabled"`
}

type AdvancedConfig struct {
	Mode              string                  `yaml:"mode"`
	DeploymentProfile string                  `yaml:"deployment_profile"`
	NodeRole          string                  `yaml:"node_role"`
	Paths             AdvancedPaths           `yaml:"paths"`
	License           AdvancedLicense         `yaml:"license"`
	ServerIdentity    ServerIdentity          `yaml:"server_identity"`
	Safety            AdvancedSafety          `yaml:"safety"`
	ServerProtection  ServerProtection        `yaml:"server_protection"`
	WebsiteProtection WebsiteProtection       `yaml:"website_protection"`
	Sites             []AdvancedSite          `yaml:"sites"`
	BackendPools      []BackendPool           `yaml:"backend_pools"`
	ResourceGovernor  ResourceGovernorConfig  `yaml:"resource_governor"`
	Updates           AdvancedUpdates         `yaml:"updates"`
	RuntimeSecurity   AdvancedRuntimeSecurity `yaml:"runtime_security"`
	Telemetry         AdvancedTelemetry       `yaml:"telemetry"`
}

type AdvancedPaths struct {
	StateDir          string `yaml:"state_dir"`
	LastGoodConfigDir string `yaml:"last_good_config_dir"`
}

type AdvancedSafety struct {
	DryRunBeforeApply                 bool `yaml:"dry_run_before_apply"`
	RequireAdminIPBeforeFirewallApply bool `yaml:"require_admin_ip_before_firewall_apply"`
	RollbackTimerSeconds              int  `yaml:"rollback_timer_seconds"`
	RequireLocalConsoleWarning        bool `yaml:"require_local_console_warning"`
}

type ServerProtection struct {
	Interfaces []string       `yaml:"interfaces"`
	XDP        AdvancedXDP    `yaml:"xdp"`
	DDOS       ServerDDOS     `yaml:"ddos"`
	Nftables   NftablesConfig `yaml:"nftables"`
}

type AdvancedXDP struct {
	Enabled             bool   `yaml:"enabled"`
	Mode                string `yaml:"mode"`
	ProgramPath         string `yaml:"program_path"`
	Section             string `yaml:"section"`
	DropPrivateSourceIP bool   `yaml:"drop_private_source_ip"`
	DropMalformed       bool   `yaml:"drop_malformed"`
	DropFragments       bool   `yaml:"drop_fragments"`
	AllowlistFile       string `yaml:"allowlist_file"`
	BlocklistFile       string `yaml:"blocklist_file"`
}

type ServerDDOS struct {
	PerIPPPS              int `yaml:"per_ip_pps"`
	PerSubnet24PPS        int `yaml:"per_subnet24_pps"`
	SynPerIPPerSecond     int `yaml:"syn_per_ip_per_second"`
	UDPPerIPPerSecond     int `yaml:"udp_per_ip_per_second"`
	ICMPPerIPPerSecond    int `yaml:"icmp_per_ip_per_second"`
	TemporaryBlockSeconds int `yaml:"temporary_block_seconds"`
	GreylistSeconds       int `yaml:"greylist_seconds"`
}

type NftablesConfig struct {
	Enabled          bool     `yaml:"enabled"`
	ProtectConntrack bool     `yaml:"protect_conntrack"`
	AllowPorts       []int    `yaml:"allow_ports"`
	SSHAdminOnly     bool     `yaml:"ssh_admin_only"`
	AdminIPs         []string `yaml:"admin_ips"`
}

type WebsiteProtection struct {
	Enabled    bool             `yaml:"enabled"`
	Cloudflare CloudflareConfig `yaml:"cloudflare"`
	WAF        WebsiteWAFConfig `yaml:"waf"`
	Bot        WebsiteBotConfig `yaml:"bot"`
}

type CloudflareConfig struct {
	Enabled               bool   `yaml:"enabled"`
	RequireProxiedTraffic bool   `yaml:"require_proxied_traffic"`
	BlockDirectOriginHTTP bool   `yaml:"block_direct_origin_http"`
	IPv4File              string `yaml:"ips_v4_file"`
	IPv6File              string `yaml:"ips_v6_file"`
}

type WebsiteWAFConfig struct {
	Enabled          bool   `yaml:"enabled"`
	Engine           string `yaml:"engine"`
	OWASPCRS         bool   `yaml:"owasp_crs"`
	AnomalyThreshold int    `yaml:"anomaly_threshold"`
}

type WebsiteBotConfig struct {
	Enabled         bool `yaml:"enabled"`
	CookieChallenge bool `yaml:"cookie_challenge"`
	JSChallenge     bool `yaml:"js_challenge"`
	ProofOfWork     bool `yaml:"proof_of_work"`
	ScoreChallenge  int  `yaml:"score_challenge"`
	ScoreBlock      int  `yaml:"score_block"`
}

type AdvancedUpdates struct {
	Enabled                     bool   `yaml:"enabled"`
	Channel                     string `yaml:"channel"`
	ManifestURL                 string `yaml:"manifest_url"`
	RequireSignedManifest       bool   `yaml:"require_signed_manifest"`
	AutoRollbackOnHealthFailure bool   `yaml:"auto_rollback_on_health_failure"`
}

type AdvancedRuntimeSecurity struct {
	Enabled                  bool                  `yaml:"enabled"`
	AuditProcessExec         bool                  `yaml:"audit_process_exec"`
	FileIntegrity            AdvancedFileIntegrity `yaml:"file_integrity"`
	AlertWhenWebUserExecutes []string              `yaml:"alert_when_web_user_executes"`
}

type AdvancedFileIntegrity struct {
	Enabled bool     `yaml:"enabled"`
	Paths   []string `yaml:"paths"`
}

type AdvancedTelemetry struct {
	Enabled      bool                 `yaml:"enabled"`
	HealthReport AdvancedHealthReport `yaml:"health_report"`
	Privacy      AdvancedPrivacy      `yaml:"privacy"`
}

type AdvancedHealthReport struct {
	Enabled             bool `yaml:"enabled"`
	SendIntervalSeconds int  `yaml:"send_interval_seconds"`
}

type AdvancedPrivacy struct {
	SendRequestBody         bool `yaml:"send_request_body"`
	SendCookie              bool `yaml:"send_cookie"`
	SendAuthorizationHeader bool `yaml:"send_authorization_header"`
	SendRawClientIP         bool `yaml:"send_raw_client_ip"`
	HashClientIP            bool `yaml:"hash_client_ip"`
	RedactSecrets           bool `yaml:"redact_secrets"`
}

type AdvancedLicense struct {
	File                string `yaml:"file"`
	ProviderPublicKey   string `yaml:"provider_public_key"`
	RequireValidLicense bool   `yaml:"require_valid_license"`
	AllowGracePeriod    bool   `yaml:"allow_grace_period"`
}

type ServerIdentity struct {
	UseMachineID      bool   `yaml:"use_machine_id"`
	UsePrimaryMAC     bool   `yaml:"use_primary_mac"`
	UseAllMACsHash    bool   `yaml:"use_all_macs_hash"`
	FingerprintSaltID string `yaml:"fingerprint_salt_id"`
}

type AdvancedSite struct {
	ID                 string          `yaml:"id"`
	Domains            []string        `yaml:"domains"`
	TLS                AdvancedTLS     `yaml:"tls"`
	Policy             string          `yaml:"policy"`
	DefaultBackendPool string          `yaml:"default_backend_pool"`
	Routes             []AdvancedRoute `yaml:"routes"`
}

type AdvancedTLS struct {
	OriginMode      string `yaml:"origin_mode"`
	CertificateFile string `yaml:"certificate_file"`
	PrivateKeyFile  string `yaml:"private_key_file"`
}

type AdvancedRoute struct {
	Path              string   `yaml:"path"`
	BackendPool       string   `yaml:"backend_pool"`
	RPMPerIP          int      `yaml:"rpm_per_ip"`
	CacheSeconds      int      `yaml:"cache_seconds"`
	MaxBodyMB         int      `yaml:"max_body_mb"`
	WAFExcludeRuleIDs []string `yaml:"waf_exclude_rules"`
}

type BackendPool struct {
	ID        string     `yaml:"id"`
	Upstreams []Upstream `yaml:"upstreams"`
}

type Upstream struct {
	ID  string `yaml:"id"`
	URL string `yaml:"url"`
}

type ProviderConfig struct {
	Provider ProviderSection `yaml:"provider"`
	Storage  StorageSection  `yaml:"storage"`
	Licenses LicenseSection  `yaml:"licenses"`
	Updates  ProviderUpdates `yaml:"updates"`
}

type ProviderSection struct {
	Name           string `yaml:"name"`
	NodeRole       string `yaml:"node_role"`
	PublicBaseURL  string `yaml:"public_base_url"`
	SigningKeyFile string `yaml:"signing_key_file"`
	PublicKeyFile  string `yaml:"public_key_file"`
}

type StorageSection struct {
	Driver  string `yaml:"driver"`
	RootDir string `yaml:"root_dir"`
}

type LicenseSection struct {
	DefaultGraceDays int                    `yaml:"default_grace_days"`
	Plans            map[string]LicensePlan `yaml:"plans"`
}

type LicensePlan struct {
	AllowedModes []string `yaml:"allowed_modes"`
	Features     []string `yaml:"features"`
}

type ProviderUpdates struct {
	Channels               []string `yaml:"channels"`
	RequireSignedArtifacts bool     `yaml:"require_signed_artifacts"`
	RollbackRetention      int      `yaml:"rollback_retention"`
}
