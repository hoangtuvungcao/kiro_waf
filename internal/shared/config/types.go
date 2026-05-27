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
	SourceKind   Kind
	Mode         string
	Plan         string
	License      RuntimeLicense
	Identity     RuntimeIdentity
	AdminCIDRs   []string
	Interface    string
	SSHPort      int
	Cloudflare   bool
	TLSMode      string
	BackendPools []RuntimeBackendPool
	Sites        []RuntimeSite
	Protection   RuntimeProtection
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
	DefaultBackendPool string
	Routes             []RuntimeRoute
}

type RuntimeRoute struct {
	Path        string
	BackendPool string
	Protection  string
}

type RuntimeProtection struct {
	Profile        string
	WAF            bool
	Bot            bool
	AutoAttackMode bool
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
	Mode              string          `yaml:"mode"`
	DeploymentProfile string          `yaml:"deployment_profile"`
	NodeRole          string          `yaml:"node_role"`
	License           AdvancedLicense `yaml:"license"`
	ServerIdentity    ServerIdentity  `yaml:"server_identity"`
	Sites             []AdvancedSite  `yaml:"sites"`
	BackendPools      []BackendPool   `yaml:"backend_pools"`
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
	Policy             string          `yaml:"policy"`
	DefaultBackendPool string          `yaml:"default_backend_pool"`
	Routes             []AdvancedRoute `yaml:"routes"`
}

type AdvancedRoute struct {
	Path        string `yaml:"path"`
	BackendPool string `yaml:"backend_pool"`
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
	DefaultGraceDays int `yaml:"default_grace_days"`
}
