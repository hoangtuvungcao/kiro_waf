package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func LoadRuntimeFile(path string) (RuntimeConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return RuntimeConfig{}, err
	}

	var probe map[string]any
	if err := yaml.Unmarshal(raw, &probe); err != nil {
		return RuntimeConfig{}, fmt.Errorf("parse yaml: %w", err)
	}
	if _, ok := probe["provider"]; ok {
		return RuntimeConfig{}, fmt.Errorf("provider config cannot be expanded as protected-server runtime config")
	}
	if role, _ := probe["node_role"].(string); role == "protected_server" {
		var cfg AdvancedConfig
		if err := yaml.Unmarshal(raw, &cfg); err != nil {
			return RuntimeConfig{}, err
		}
		if err := ValidateAdvanced(cfg); err != nil {
			return RuntimeConfig{}, err
		}
		return ExpandAdvanced(cfg)
	}

	var cfg TenantConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return RuntimeConfig{}, err
	}
	if err := ValidateTenant(cfg); err != nil {
		return RuntimeConfig{}, err
	}
	return ExpandTenant(cfg)
}

func ExpandTenant(cfg TenantConfig) (RuntimeConfig, error) {
	if err := ValidateTenant(cfg); err != nil {
		return RuntimeConfig{}, err
	}

	runtime := RuntimeConfig{
		SourceKind: KindTenant,
		Mode:       cfg.Mode,
		Plan:       cfg.Plan,
		Paths: RuntimePaths{
			StateDir:          "/var/lib/kiro",
			LastGoodConfigDir: "/var/lib/kiro/last-good-config",
		},
		Safety: RuntimeSafety{
			DryRunBeforeApply:                 true,
			RequireAdminIPBeforeFirewallApply: true,
			RollbackTimerSeconds:              60,
			RequireLocalConsoleWarning:        true,
		},
		AdminCIDRs: append([]string(nil), cfg.Admin.AllowIPs...),
		Interface:  cfg.Server.Interface,
		SSHPort:    cfg.Server.SSHPort,
		Cloudflare: cfg.Website.Cloudflare,
		TLSMode:    cfg.Website.TLSMode,
		Firewall: RuntimeFirewall{
			Enabled:                 true,
			ProtectConntrack:        true,
			AllowPorts:              tenantAllowPorts(cfg),
			SSHAdminOnly:            true,
			AdminCIDRs:              append([]string(nil), cfg.Admin.AllowIPs...),
			TemporaryBlockSeconds:   900,
			RequireAdminBeforeApply: true,
		},
		CFOriginLock: RuntimeCloudflareOriginLock{
			Enabled:               cfg.Website.Cloudflare,
			RequireProxiedTraffic: cfg.Website.Cloudflare,
			BlockDirectOriginHTTP: cfg.Website.Cloudflare,
			IPv4File:              "rules/cloudflare/ips-v4.txt",
			IPv6File:              "rules/cloudflare/ips-v6.txt",
		},
		Protection: RuntimeProtection{
			Profile:        cfg.Protection.Profile,
			WAF:            cfg.Protection.WAF,
			Bot:            cfg.Protection.Bot,
			AutoAttackMode: cfg.Protection.AutoAttackMode,
		},
	}

	if runtime.SSHPort == 0 {
		runtime.SSHPort = 22
	}
	if runtime.Mode == "server" && !cfg.Website.Enabled {
		return runtime, nil
	}

	poolByBackend := map[string]string{}
	for siteIndex, site := range cfg.Website.Sites {
		defaultPool := poolIDForBackend(poolByBackend, &runtime, site.Backend)
		runtimeSite := RuntimeSite{
			ID:                 fmt.Sprintf("site_%d", siteIndex+1),
			Domains:            append([]string(nil), site.Domains...),
			TLSMode:            cfg.Website.TLSMode,
			CertFile:           cfg.Website.CertFile,
			KeyFile:            cfg.Website.KeyFile,
			DefaultBackendPool: defaultPool,
		}
		for routeIndex, route := range site.Routes {
			routePool := poolIDForBackend(poolByBackend, &runtime, route.Backend)
			runtimeSite.Routes = append(runtimeSite.Routes, RuntimeRoute{
				Path:        route.Path,
				BackendPool: routePool,
				Protection:  route.Protection,
			})
			if runtimeSite.Routes[routeIndex].Protection == "" {
				runtimeSite.Routes[routeIndex].Protection = cfg.Protection.Profile
			}
		}
		runtime.Sites = append(runtime.Sites, runtimeSite)
	}

	if err := ValidateRuntime(runtime); err != nil {
		return RuntimeConfig{}, err
	}
	return runtime, nil
}

func ExpandAdvanced(cfg AdvancedConfig) (RuntimeConfig, error) {
	if err := ValidateAdvanced(cfg); err != nil {
		return RuntimeConfig{}, err
	}
	runtime := RuntimeConfig{
		SourceKind: KindAdvanced,
		Mode:       cfg.Mode,
		Plan:       cfg.DeploymentProfile,
		Paths: RuntimePaths{
			StateDir:          cfg.Paths.StateDir,
			LastGoodConfigDir: cfg.Paths.LastGoodConfigDir,
		},
		Safety: RuntimeSafety{
			DryRunBeforeApply:                 cfg.Safety.DryRunBeforeApply,
			RequireAdminIPBeforeFirewallApply: cfg.Safety.RequireAdminIPBeforeFirewallApply,
			RollbackTimerSeconds:              cfg.Safety.RollbackTimerSeconds,
			RequireLocalConsoleWarning:        cfg.Safety.RequireLocalConsoleWarning,
		},
		License: RuntimeLicense{
			File:                cfg.License.File,
			ProviderPublicKey:   cfg.License.ProviderPublicKey,
			RequireValidLicense: cfg.License.RequireValidLicense,
			AllowGracePeriod:    cfg.License.AllowGracePeriod,
		},
		Identity: RuntimeIdentity{
			UseMachineID:      cfg.ServerIdentity.UseMachineID,
			UsePrimaryMAC:     cfg.ServerIdentity.UsePrimaryMAC,
			UseAllMACsHash:    cfg.ServerIdentity.UseAllMACsHash,
			FingerprintSaltID: cfg.ServerIdentity.FingerprintSaltID,
		},
		AdminCIDRs: append([]string(nil), cfg.ServerProtection.Nftables.AdminIPs...),
		SSHPort:    sshPortFromAllowPorts(cfg.ServerProtection.Nftables.AllowPorts),
		Firewall: RuntimeFirewall{
			Enabled:                 cfg.ServerProtection.Nftables.Enabled,
			ProtectConntrack:        cfg.ServerProtection.Nftables.ProtectConntrack,
			AllowPorts:              append([]int(nil), cfg.ServerProtection.Nftables.AllowPorts...),
			SSHAdminOnly:            cfg.ServerProtection.Nftables.SSHAdminOnly,
			AdminCIDRs:              append([]string(nil), cfg.ServerProtection.Nftables.AdminIPs...),
			TemporaryBlockSeconds:   cfg.ServerProtection.DDOS.TemporaryBlockSeconds,
			RequireAdminBeforeApply: cfg.Safety.RequireAdminIPBeforeFirewallApply,
		},
		CFOriginLock: RuntimeCloudflareOriginLock{
			Enabled:               cfg.WebsiteProtection.Cloudflare.Enabled,
			RequireProxiedTraffic: cfg.WebsiteProtection.Cloudflare.RequireProxiedTraffic,
			BlockDirectOriginHTTP: cfg.WebsiteProtection.Cloudflare.BlockDirectOriginHTTP,
			IPv4File:              cfg.WebsiteProtection.Cloudflare.IPv4File,
			IPv6File:              cfg.WebsiteProtection.Cloudflare.IPv6File,
		},
	}
	if len(cfg.ServerProtection.Interfaces) > 0 {
		runtime.Interface = cfg.ServerProtection.Interfaces[0]
	}
	if runtime.SSHPort == 0 {
		runtime.SSHPort = 22
	}
	if runtime.Paths.StateDir == "" {
		runtime.Paths.StateDir = "/var/lib/kiro"
	}
	if runtime.Paths.LastGoodConfigDir == "" {
		runtime.Paths.LastGoodConfigDir = runtime.Paths.StateDir + "/last-good-config"
	}
	if runtime.Safety.RollbackTimerSeconds == 0 {
		runtime.Safety.RollbackTimerSeconds = 60
	}
	if runtime.Firewall.TemporaryBlockSeconds == 0 {
		runtime.Firewall.TemporaryBlockSeconds = 900
	}
	for _, pool := range cfg.BackendPools {
		runtimePool := RuntimeBackendPool{ID: pool.ID}
		for _, upstream := range pool.Upstreams {
			runtimePool.Upstreams = append(runtimePool.Upstreams, RuntimeUpstream{
				ID:  upstream.ID,
				URL: upstream.URL,
			})
		}
		runtime.BackendPools = append(runtime.BackendPools, runtimePool)
	}
	for _, site := range cfg.Sites {
		runtimeSite := RuntimeSite{
			ID:                 site.ID,
			Domains:            append([]string(nil), site.Domains...),
			TLSMode:            firstNonEmpty(site.TLS.OriginMode, runtime.TLSMode, "flexible_http"),
			CertFile:           site.TLS.CertificateFile,
			KeyFile:            site.TLS.PrivateKeyFile,
			DefaultBackendPool: site.DefaultBackendPool,
		}
		for _, route := range site.Routes {
			runtimeSite.Routes = append(runtimeSite.Routes, RuntimeRoute{
				Path:         route.Path,
				BackendPool:  route.BackendPool,
				RPMPerIP:     route.RPMPerIP,
				CacheSeconds: route.CacheSeconds,
				MaxBodyMB:    route.MaxBodyMB,
			})
		}
		runtime.Sites = append(runtime.Sites, runtimeSite)
	}
	if err := ValidateRuntime(runtime); err != nil {
		return RuntimeConfig{}, err
	}
	return runtime, nil
}

func ValidateRuntime(cfg RuntimeConfig) error {
	if !allowedModes[cfg.Mode] {
		return fmt.Errorf("invalid runtime mode %q", cfg.Mode)
	}
	pools := map[string]bool{}
	for _, pool := range cfg.BackendPools {
		if pool.ID == "" {
			return fmt.Errorf("runtime backend pool id is required")
		}
		if pools[pool.ID] {
			return fmt.Errorf("duplicate runtime backend pool %q", pool.ID)
		}
		pools[pool.ID] = true
		if len(pool.Upstreams) == 0 {
			return fmt.Errorf("runtime backend pool %q must have upstreams", pool.ID)
		}
	}
	domains := map[string]string{}
	for _, site := range cfg.Sites {
		if site.ID == "" {
			return fmt.Errorf("runtime site id is required")
		}
		if !pools[site.DefaultBackendPool] {
			return fmt.Errorf("runtime site %q references unknown backend pool %q", site.ID, site.DefaultBackendPool)
		}
		for _, domain := range site.Domains {
			if prev, ok := domains[domain]; ok {
				return fmt.Errorf("duplicate domain %q in sites %q and %q", domain, prev, site.ID)
			}
			domains[domain] = site.ID
		}
		for _, route := range site.Routes {
			if route.Path == "" || route.Path[0] != '/' {
				return fmt.Errorf("runtime site %q route path must start with /", site.ID)
			}
			if !pools[route.BackendPool] {
				return fmt.Errorf("runtime site %q route %q references unknown backend pool %q", site.ID, route.Path, route.BackendPool)
			}
		}
	}
	return nil
}

func poolIDForBackend(index map[string]string, runtime *RuntimeConfig, backend string) string {
	if id, ok := index[backend]; ok {
		return id
	}
	id := fmt.Sprintf("backend_pool_%d", len(index)+1)
	index[backend] = id
	runtime.BackendPools = append(runtime.BackendPools, RuntimeBackendPool{
		ID: id,
		Upstreams: []RuntimeUpstream{{
			ID:  "upstream_1",
			URL: backend,
		}},
	})
	return id
}

func tenantAllowPorts(cfg TenantConfig) []int {
	if len(cfg.Server.AllowPorts) > 0 {
		return append([]int(nil), cfg.Server.AllowPorts...)
	}
	ports := []int{cfg.Server.SSHPort}
	if cfg.Mode == "full" || cfg.Website.Enabled {
		ports = append(ports, 80, 443)
	}
	return ports
}

func sshPortFromAllowPorts(ports []int) int {
	for _, port := range ports {
		if port == 22 {
			return 22
		}
	}
	return 22
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
