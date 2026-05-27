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
		AdminCIDRs: append([]string(nil), cfg.Admin.AllowIPs...),
		Interface:  cfg.Server.Interface,
		SSHPort:    cfg.Server.SSHPort,
		Cloudflare: cfg.Website.Cloudflare,
		TLSMode:    cfg.Website.TLSMode,
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
			DefaultBackendPool: site.DefaultBackendPool,
		}
		for _, route := range site.Routes {
			runtimeSite.Routes = append(runtimeSite.Routes, RuntimeRoute{
				Path:        route.Path,
				BackendPool: route.BackendPool,
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
