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
		XDP: defaultRuntimeXDP(false),
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
		WAF:              defaultRuntimeWAF(cfg.Protection.WAF),
		Bot:              defaultRuntimeBot(cfg.Protection.Bot, cfg.Admin.AllowIPs),
		ResourceGovernor: defaultResourceGovernor(cfg.Protection.AutoAttackMode, "/var/lib/kiro"),
		Updates:          defaultRuntimeUpdates(cfg.Updates.AutoSecurityUpdates),
		RuntimeSecurity:  defaultRuntimeSecurity(false),
		Telemetry:        defaultRuntimeTelemetry(cfg.Telemetry.Enabled),
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
		XDP: normalizeRuntimeXDP(RuntimeXDP{
			Enabled:             cfg.ServerProtection.XDP.Enabled,
			Mode:                cfg.ServerProtection.XDP.Mode,
			ProgramPath:         cfg.ServerProtection.XDP.ProgramPath,
			Section:             cfg.ServerProtection.XDP.Section,
			DropPrivateSourceIP: cfg.ServerProtection.XDP.DropPrivateSourceIP,
			DropMalformed:       cfg.ServerProtection.XDP.DropMalformed,
			DropFragments:       cfg.ServerProtection.XDP.DropFragments,
			RateLimitEnabled:    xdpRateLimitEnabled(cfg.ServerProtection.DDOS),
			WindowSeconds:       1,
			PerIPPPS:            cfg.ServerProtection.DDOS.PerIPPPS,
			PerSubnet24PPS:      cfg.ServerProtection.DDOS.PerSubnet24PPS,
			SynPPS:              cfg.ServerProtection.DDOS.SynPerIPPerSecond,
			UDPPPS:              cfg.ServerProtection.DDOS.UDPPerIPPerSecond,
			ICMPPPS:             cfg.ServerProtection.DDOS.ICMPPerIPPerSecond,
			AllowlistFile:       cfg.ServerProtection.XDP.AllowlistFile,
			BlocklistFile:       cfg.ServerProtection.XDP.BlocklistFile,
		}),
		CFOriginLock: RuntimeCloudflareOriginLock{
			Enabled:               cfg.WebsiteProtection.Cloudflare.Enabled,
			RequireProxiedTraffic: cfg.WebsiteProtection.Cloudflare.RequireProxiedTraffic,
			BlockDirectOriginHTTP: cfg.WebsiteProtection.Cloudflare.BlockDirectOriginHTTP,
			IPv4File:              cfg.WebsiteProtection.Cloudflare.IPv4File,
			IPv6File:              cfg.WebsiteProtection.Cloudflare.IPv6File,
		},
		WAF: normalizeRuntimeWAF(RuntimeWAF{
			Enabled:          cfg.WebsiteProtection.WAF.Enabled,
			Engine:           cfg.WebsiteProtection.WAF.Engine,
			OWASPCRS:         cfg.WebsiteProtection.WAF.OWASPCRS,
			AnomalyThreshold: cfg.WebsiteProtection.WAF.AnomalyThreshold,
		}),
		Bot: normalizeRuntimeBot(RuntimeBot{
			Enabled:            cfg.WebsiteProtection.Bot.Enabled,
			CookieChallenge:    cfg.WebsiteProtection.Bot.CookieChallenge,
			JSChallenge:        cfg.WebsiteProtection.Bot.JSChallenge,
			ProofOfWork:        cfg.WebsiteProtection.Bot.ProofOfWork,
			ScoreChallenge:     cfg.WebsiteProtection.Bot.ScoreChallenge,
			ScoreBlock:         cfg.WebsiteProtection.Bot.ScoreBlock,
			TrustedClientCIDRs: append([]string(nil), cfg.ServerProtection.Nftables.AdminIPs...),
		}),
		Updates: normalizeRuntimeUpdates(RuntimeUpdates{
			Enabled:                     cfg.Updates.Enabled,
			Channel:                     cfg.Updates.Channel,
			ManifestURL:                 cfg.Updates.ManifestURL,
			RequireSignedManifest:       cfg.Updates.RequireSignedManifest,
			AutoRollbackOnHealthFailure: cfg.Updates.AutoRollbackOnHealthFailure,
		}),
		RuntimeSecurity: normalizeRuntimeSecurity(RuntimeSecurity{
			Enabled:              cfg.RuntimeSecurity.Enabled,
			AuditProcessExec:     cfg.RuntimeSecurity.AuditProcessExec,
			FileIntegrityEnabled: cfg.RuntimeSecurity.FileIntegrity.Enabled,
			FileIntegrityPaths:   append([]string(nil), cfg.RuntimeSecurity.FileIntegrity.Paths...),
			AlertProcessNames:    append([]string(nil), cfg.RuntimeSecurity.AlertWhenWebUserExecutes...),
		}),
		Telemetry: normalizeRuntimeTelemetry(RuntimeTelemetry{
			Enabled:                   cfg.Telemetry.Enabled,
			HealthReportEnabled:       cfg.Telemetry.HealthReport.Enabled,
			HealthSendIntervalSeconds: cfg.Telemetry.HealthReport.SendIntervalSeconds,
			Privacy: RuntimePrivacy{
				SendRequestBody:         cfg.Telemetry.Privacy.SendRequestBody,
				SendCookie:              cfg.Telemetry.Privacy.SendCookie,
				SendAuthorizationHeader: cfg.Telemetry.Privacy.SendAuthorizationHeader,
				SendRawClientIP:         cfg.Telemetry.Privacy.SendRawClientIP,
				HashClientIP:            cfg.Telemetry.Privacy.HashClientIP,
				RedactSecrets:           cfg.Telemetry.Privacy.RedactSecrets,
			},
		}),
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
	runtime.ResourceGovernor = normalizeResourceGovernor(cfg.ResourceGovernor, runtime.Paths.StateDir)
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
				Path:              route.Path,
				BackendPool:       route.BackendPool,
				RPMPerIP:          route.RPMPerIP,
				CacheSeconds:      route.CacheSeconds,
				MaxBodyMB:         route.MaxBodyMB,
				WAFExcludeRuleIDs: append([]string(nil), route.WAFExcludeRuleIDs...),
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

func defaultRuntimeWAF(enabled bool) RuntimeWAF {
	return RuntimeWAF{
		Enabled:          enabled,
		Engine:           "coraza",
		OWASPCRS:         enabled,
		AnomalyThreshold: 5,
	}
}

func defaultRuntimeXDP(enabled bool) RuntimeXDP {
	return RuntimeXDP{
		Enabled:       enabled,
		Mode:          "generic",
		ProgramPath:   "/usr/lib/kiro/xdp/kiro_xdp_drop.o",
		Section:       "xdp",
		WindowSeconds: 1,
	}
}

func normalizeRuntimeXDP(runtime RuntimeXDP) RuntimeXDP {
	if !runtime.Enabled {
		return runtime
	}
	defaults := defaultRuntimeXDP(true)
	if runtime.Mode == "" {
		runtime.Mode = defaults.Mode
	}
	if runtime.ProgramPath == "" {
		runtime.ProgramPath = defaults.ProgramPath
	}
	if runtime.Section == "" {
		runtime.Section = defaults.Section
	}
	if runtime.WindowSeconds <= 0 {
		runtime.WindowSeconds = defaults.WindowSeconds
	}
	return runtime
}

func xdpRateLimitEnabled(ddos ServerDDOS) bool {
	return ddos.PerIPPPS > 0 ||
		ddos.PerSubnet24PPS > 0 ||
		ddos.SynPerIPPerSecond > 0 ||
		ddos.UDPPerIPPerSecond > 0 ||
		ddos.ICMPPerIPPerSecond > 0
}

func normalizeRuntimeWAF(runtime RuntimeWAF) RuntimeWAF {
	if !runtime.Enabled {
		return runtime
	}
	defaults := defaultRuntimeWAF(true)
	if runtime.Engine == "" {
		runtime.Engine = defaults.Engine
	}
	if runtime.AnomalyThreshold == 0 {
		runtime.AnomalyThreshold = defaults.AnomalyThreshold
	}
	return runtime
}

func defaultRuntimeBot(enabled bool, trustedCIDRs []string) RuntimeBot {
	return RuntimeBot{
		Enabled:             enabled,
		CookieChallenge:     enabled,
		ScoreChallenge:      50,
		ScoreBlock:          80,
		ChallengeCookieName: "kiro_challenge",
		TrustedClientCIDRs:  append([]string(nil), trustedCIDRs...),
	}
}

func normalizeRuntimeBot(runtime RuntimeBot) RuntimeBot {
	if !runtime.Enabled {
		return runtime
	}
	defaults := defaultRuntimeBot(true, runtime.TrustedClientCIDRs)
	if runtime.ScoreChallenge == 0 {
		runtime.ScoreChallenge = defaults.ScoreChallenge
	}
	if runtime.ScoreBlock == 0 {
		runtime.ScoreBlock = defaults.ScoreBlock
	}
	if runtime.ChallengeCookieName == "" {
		runtime.ChallengeCookieName = defaults.ChallengeCookieName
	}
	return runtime
}

func defaultRuntimeUpdates(enabled bool) RuntimeUpdates {
	return RuntimeUpdates{
		Enabled:                     enabled,
		Channel:                     "stable",
		RequireSignedManifest:       true,
		AutoRollbackOnHealthFailure: true,
	}
}

func normalizeRuntimeUpdates(runtime RuntimeUpdates) RuntimeUpdates {
	if !runtime.Enabled {
		return runtime
	}
	defaults := defaultRuntimeUpdates(true)
	if runtime.Channel == "" {
		runtime.Channel = defaults.Channel
	}
	if !runtime.RequireSignedManifest {
		runtime.RequireSignedManifest = defaults.RequireSignedManifest
	}
	if !runtime.AutoRollbackOnHealthFailure {
		runtime.AutoRollbackOnHealthFailure = defaults.AutoRollbackOnHealthFailure
	}
	return runtime
}

func defaultRuntimeSecurity(enabled bool) RuntimeSecurity {
	return RuntimeSecurity{
		Enabled:              enabled,
		AuditProcessExec:     enabled,
		FileIntegrityEnabled: enabled,
		FileIntegrityPaths:   []string{"/etc/kiro"},
		AlertProcessNames:    []string{"sh", "bash", "curl", "wget", "nc", "python", "perl"},
		WebUsers:             []string{"www-data", "nginx", "apache", "httpd"},
	}
}

func normalizeRuntimeSecurity(runtime RuntimeSecurity) RuntimeSecurity {
	if !runtime.Enabled {
		return runtime
	}
	defaults := defaultRuntimeSecurity(true)
	if len(runtime.AlertProcessNames) == 0 {
		runtime.AlertProcessNames = defaults.AlertProcessNames
	}
	if len(runtime.WebUsers) == 0 {
		runtime.WebUsers = defaults.WebUsers
	}
	return runtime
}

func defaultRuntimeTelemetry(enabled bool) RuntimeTelemetry {
	return RuntimeTelemetry{
		Enabled:                   enabled,
		HealthReportEnabled:       enabled,
		HealthSendIntervalSeconds: 3600,
		Privacy: RuntimePrivacy{
			HashClientIP:  true,
			RedactSecrets: true,
		},
	}
}

func normalizeRuntimeTelemetry(runtime RuntimeTelemetry) RuntimeTelemetry {
	defaults := defaultRuntimeTelemetry(runtime.Enabled)
	if runtime.HealthReportEnabled && runtime.HealthSendIntervalSeconds == 0 {
		runtime.HealthSendIntervalSeconds = defaults.HealthSendIntervalSeconds
	}
	if !runtime.Privacy.SendRequestBody &&
		!runtime.Privacy.SendCookie &&
		!runtime.Privacy.SendAuthorizationHeader &&
		!runtime.Privacy.SendRawClientIP &&
		!runtime.Privacy.HashClientIP &&
		!runtime.Privacy.RedactSecrets {
		runtime.Privacy = defaults.Privacy
	}
	return runtime
}

func defaultResourceGovernor(enabled bool, stateDir string) ResourceGovernorConfig {
	if stateDir == "" {
		stateDir = "/var/lib/kiro"
	}
	return ResourceGovernorConfig{
		Enabled: enabled,
		Baseline: ResourceGovernorBaseline{
			Enabled:      enabled,
			LearningDays: 7,
			MinSamples:   1000,
			StoreFile:    stateDir + "/baseline/baseline.json",
		},
		Hysteresis: ResourceGovernorHysteresis{
			Enabled:                enabled,
			MinLevelHoldSeconds:    120,
			CooldownSeconds:        600,
			RequireRecoverySamples: 5,
		},
		Levels: ResourceGovernorLevels{
			Elevated: ResourceGovernorLevelThreshold{
				CPUPercent:          75,
				RAMAvailablePercent: 20,
				ConntrackPercent:    60,
				BackendLatencyMS:    800,
			},
			Attack: ResourceGovernorLevelThreshold{
				CPUPercent:          85,
				RAMAvailablePercent: 15,
				ConntrackPercent:    75,
				BackendLatencyMS:    1500,
			},
			Lockdown: ResourceGovernorLevelThreshold{
				CPUPercent:          95,
				RAMAvailablePercent: 8,
				ConntrackPercent:    90,
				BackendLatencyMS:    3000,
			},
		},
		Actions: ResourceGovernorActions{
			Elevated: ResourceGovernorElevatedActions{
				TightenRateLimits:            true,
				EnableChallengeForNewClients: true,
				IncreaseCache:                true,
			},
			Attack: ResourceGovernorAttackActions{
				TemporaryBlockBadClients: true,
				DisableExpensiveRoutes:   true,
				LowerTimeouts:            true,
			},
			Lockdown: ResourceGovernorLockdownActions{
				AllowAdminAndKnownClientsOnly: true,
				ProtectBackendFirst:           true,
			},
		},
	}
}

func normalizeResourceGovernor(cfg ResourceGovernorConfig, stateDir string) ResourceGovernorConfig {
	if !cfg.Enabled {
		return cfg
	}
	defaults := defaultResourceGovernor(true, stateDir)
	if cfg.Baseline.StoreFile == "" {
		cfg.Baseline.StoreFile = defaults.Baseline.StoreFile
	}
	if cfg.Baseline.Enabled {
		if cfg.Baseline.LearningDays == 0 {
			cfg.Baseline.LearningDays = defaults.Baseline.LearningDays
		}
		if cfg.Baseline.MinSamples == 0 {
			cfg.Baseline.MinSamples = defaults.Baseline.MinSamples
		}
	}
	if zeroThreshold(cfg.Levels.Elevated) {
		cfg.Levels.Elevated = defaults.Levels.Elevated
	}
	if zeroThreshold(cfg.Levels.Attack) {
		cfg.Levels.Attack = defaults.Levels.Attack
	}
	if zeroThreshold(cfg.Levels.Lockdown) {
		cfg.Levels.Lockdown = defaults.Levels.Lockdown
	}
	if zeroElevatedActions(cfg.Actions.Elevated) {
		cfg.Actions.Elevated = defaults.Actions.Elevated
	}
	if zeroAttackActions(cfg.Actions.Attack) {
		cfg.Actions.Attack = defaults.Actions.Attack
	}
	if zeroLockdownActions(cfg.Actions.Lockdown) {
		cfg.Actions.Lockdown = defaults.Actions.Lockdown
	}
	return cfg
}

func zeroThreshold(threshold ResourceGovernorLevelThreshold) bool {
	return threshold.CPUPercent == 0 &&
		threshold.RAMAvailablePercent == 0 &&
		threshold.Load1 == 0 &&
		threshold.ConntrackPercent == 0 &&
		threshold.BackendLatencyMS == 0
}

func zeroElevatedActions(actions ResourceGovernorElevatedActions) bool {
	return !actions.TightenRateLimits &&
		!actions.EnableChallengeForNewClients &&
		!actions.IncreaseCache
}

func zeroAttackActions(actions ResourceGovernorAttackActions) bool {
	return !actions.TemporaryBlockBadClients &&
		!actions.DisableExpensiveRoutes &&
		!actions.LowerTimeouts
}

func zeroLockdownActions(actions ResourceGovernorLockdownActions) bool {
	return !actions.AllowAdminAndKnownClientsOnly &&
		!actions.ProtectBackendFirst
}
