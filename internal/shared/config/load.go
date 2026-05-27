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
	allowedModes    = map[string]bool{"server": true, "full": true}
	allowedProfiles = map[string]bool{"light": true, "balanced": true, "strict": true, "lockdown": true}
	allowedTLSModes = map[string]bool{"flexible_http": true, "full_tls": true, "full_strict": true}
)

func CheckFile(path string) (Result, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Result{}, err
	}

	var probe map[string]any
	if err := yaml.Unmarshal(raw, &probe); err != nil {
		return Result{}, fmt.Errorf("parse yaml: %w", err)
	}

	if _, ok := probe["provider"]; ok {
		var cfg ProviderConfig
		if err := yaml.Unmarshal(raw, &cfg); err != nil {
			return Result{}, err
		}
		if err := ValidateProvider(cfg); err != nil {
			return Result{}, err
		}
		return Result{Path: path, Kind: KindProvider}, nil
	}

	if role, _ := probe["node_role"].(string); role == "protected_server" {
		var cfg AdvancedConfig
		if err := yaml.Unmarshal(raw, &cfg); err != nil {
			return Result{}, err
		}
		if err := ValidateAdvanced(cfg); err != nil {
			return Result{}, err
		}
		return Result{Path: path, Kind: KindAdvanced, Mode: cfg.Mode, Plan: cfg.DeploymentProfile}, nil
	}

	var cfg TenantConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Result{}, err
	}
	if err := ValidateTenant(cfg); err != nil {
		return Result{}, err
	}
	return Result{Path: path, Kind: KindTenant, Mode: cfg.Mode, Plan: cfg.Plan}, nil
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
