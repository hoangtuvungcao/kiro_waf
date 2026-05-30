package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"kiro_waf/cmd/kiro-cli/update"
	"kiro_waf/internal/shared/buildinfo"
	"kiro_waf/internal/shared/config"
	"kiro_waf/internal/shared/diagnostics"
	"kiro_waf/internal/shared/installer"
	"kiro_waf/internal/shared/machinefingerprint"
	"kiro_waf/internal/shared/pilot"
	"kiro_waf/internal/shared/support"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	if os.Args[1] == "version" {
		fmt.Println(buildinfo.Version)
		return
	}
	switch os.Args[1] {
	case "license":
		runLicense(os.Args[2:])
	case "status":
		runStatus(os.Args[2:])
	case "health":
		runHealth(os.Args[2:])
	case "preflight":
		runPreflight(os.Args[2:])
	case "mode":
		runMode(os.Args[2:])
	case "install":
		runInstall(os.Args[2:])
	case "update":
		runUpdate(os.Args[2:])
	case "incident":
		runIncident(os.Args[2:])
	case "pilot":
		runPilot(os.Args[2:])
	case "report":
		runReport(os.Args[2:])
	default:
		usage()
	}
}

func runLicense(args []string) {
	if len(args) == 0 || args[0] != "fingerprint" {
		usage()
	}
	fingerprintCmd := flag.NewFlagSet("kiro-cli license fingerprint", flag.ExitOnError)
	salt := fingerprintCmd.String("salt", "", "provider fingerprint salt id")
	if err := fingerprintCmd.Parse(args[1:]); err != nil {
		os.Exit(2)
	}
	snapshot, err := machinefingerprint.Collect(machinefingerprint.Options{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "fingerprint failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(snapshot.FingerprintHash(*salt))
}

func runStatus(args []string) {
	cmd := flag.NewFlagSet("kiro-cli status", flag.ExitOnError)
	configPath := cmd.String("config", "configs/kiro.example.yaml", "path to kiro config")
	if err := cmd.Parse(args); err != nil {
		os.Exit(2)
	}
	runtime := mustLoadRuntime(*configPath)
	writeJSON(diagnostics.BuildStatus(runtime, time.Time{}))
}

func runHealth(args []string) {
	cmd := flag.NewFlagSet("kiro-cli health", flag.ExitOnError)
	configPath := cmd.String("config", "configs/kiro.example.yaml", "path to kiro config")
	osRelease := cmd.String("os-release", "/etc/os-release", "path to os-release for preflight")
	writableRoot := cmd.String("preflight-writable-root", "", "optional writable root for state-dir preflight")
	skipCommands := cmd.Bool("skip-command-checks", false, "skip nft/nginx/systemctl PATH checks")
	if err := cmd.Parse(args); err != nil {
		os.Exit(2)
	}
	runtime := mustLoadRuntime(*configPath)
	preflight := diagnostics.BuildPreflight(runtime, diagnostics.PreflightOptions{
		OSReleasePath:     *osRelease,
		EffectiveUID:      os.Geteuid(),
		WritableRoot:      *writableRoot,
		SkipCommandChecks: *skipCommands,
	})
	writeJSON(diagnostics.BuildHealth(runtime, preflight, time.Time{}))
}

func runPreflight(args []string) {
	cmd := flag.NewFlagSet("kiro-cli preflight", flag.ExitOnError)
	configPath := cmd.String("config", "configs/kiro.example.yaml", "path to kiro config")
	osRelease := cmd.String("os-release", "/etc/os-release", "path to os-release for preflight")
	writableRoot := cmd.String("preflight-writable-root", "", "optional writable root for state-dir preflight")
	skipCommands := cmd.Bool("skip-command-checks", false, "skip nft/nginx/systemctl PATH checks")
	if err := cmd.Parse(args); err != nil {
		os.Exit(2)
	}
	runtime := mustLoadRuntime(*configPath)
	writeJSON(diagnostics.BuildPreflight(runtime, diagnostics.PreflightOptions{
		OSReleasePath:     *osRelease,
		EffectiveUID:      os.Geteuid(),
		WritableRoot:      *writableRoot,
		SkipCommandChecks: *skipCommands,
	}))
}

func runMode(args []string) {
	if len(args) == 0 {
		usage()
	}
	switch args[0] {
	case "show":
		cmd := flag.NewFlagSet("kiro-cli mode show", flag.ExitOnError)
		configPath := cmd.String("config", "configs/kiro.example.yaml", "path to kiro config")
		if err := cmd.Parse(args[1:]); err != nil {
			os.Exit(2)
		}
		mode, err := diagnostics.ShowMode(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mode show failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(mode)
	case "set":
		cmd := flag.NewFlagSet("kiro-cli mode set", flag.ExitOnError)
		configPath := cmd.String("config", "configs/kiro.example.yaml", "path to kiro config")
		mode := cmd.String("mode", "", "mode to set: server or full")
		if err := cmd.Parse(args[1:]); err != nil {
			os.Exit(2)
		}
		if err := diagnostics.SetModeFile(*configPath, *mode); err != nil {
			fmt.Fprintf(os.Stderr, "mode set failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("mode set: %s\n", *mode)
	default:
		usage()
	}
}

func runInstall(args []string) {
	if len(args) == 0 {
		usage()
	}
	switch args[0] {
	case "plan":
		cmd := flag.NewFlagSet("kiro-cli install plan", flag.ExitOnError)
		configPath := cmd.String("config", "configs/kiro.example.yaml", "path to kiro config")
		installRoot := cmd.String("install-root", "", "optional lab/staging root prefix")
		agentBinary := cmd.String("agent-binary", "", "source kiro-agent binary to install")
		cliBinary := cmd.String("cli-binary", "", "source kiro-cli binary to install")
		providerBinary := cmd.String("provider-binary", "", "optional source kiro-provider binary to install")
		servicePath := cmd.String("systemd-service", installer.DefaultServiceSource, "source systemd service file")
		if err := cmd.Parse(args[1:]); err != nil {
			os.Exit(2)
		}
		runtime := mustLoadRuntime(*configPath)
		plan, err := installer.BuildInstallPlan(runtime, installer.Options{
			ConfigPath:         *configPath,
			InstallRoot:        *installRoot,
			AgentBinary:        *agentBinary,
			CLIBinary:          *cliBinary,
			ProviderBinary:     *providerBinary,
			SystemdServicePath: *servicePath,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "install plan failed: %v\n", err)
			os.Exit(1)
		}
		writeJSON(plan)
	case "stage-lab":
		cmd := flag.NewFlagSet("kiro-cli install stage-lab", flag.ExitOnError)
		configPath := cmd.String("config", "configs/kiro.example.yaml", "path to kiro config")
		installRoot := cmd.String("install-root", "", "required lab/staging root prefix")
		agentBinary := cmd.String("agent-binary", "", "source kiro-agent binary to install")
		cliBinary := cmd.String("cli-binary", "", "source kiro-cli binary to install")
		providerBinary := cmd.String("provider-binary", "", "optional source kiro-provider binary to install")
		servicePath := cmd.String("systemd-service", installer.DefaultServiceSource, "source systemd service file")
		if err := cmd.Parse(args[1:]); err != nil {
			os.Exit(2)
		}
		runtime := mustLoadRuntime(*configPath)
		result, err := installer.StageLabInstall(runtime, installer.Options{
			ConfigPath:         *configPath,
			InstallRoot:        *installRoot,
			AgentBinary:        *agentBinary,
			CLIBinary:          *cliBinary,
			ProviderBinary:     *providerBinary,
			SystemdServicePath: *servicePath,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "install stage-lab failed: %v\n", err)
			os.Exit(1)
		}
		writeJSON(result)
	case "apply-lab":
		cmd := flag.NewFlagSet("kiro-cli install apply-lab", flag.ExitOnError)
		configPath := cmd.String("config", "configs/kiro.example.yaml", "path to kiro config")
		installRoot := cmd.String("install-root", "", "optional lab/staging root prefix; empty applies to real root")
		agentBinary := cmd.String("agent-binary", "", "source kiro-agent binary to install")
		cliBinary := cmd.String("cli-binary", "", "source kiro-cli binary to install")
		providerBinary := cmd.String("provider-binary", "", "optional source kiro-provider binary to install")
		servicePath := cmd.String("systemd-service", installer.DefaultServiceSource, "source systemd service file")
		ack := cmd.String("ack", "", "required value: KIRO_LAB_INSTALL_APPLY")
		osRelease := cmd.String("os-release", "/etc/os-release", "path to os-release for Ubuntu guard")
		skipOSCheck := cmd.Bool("skip-os-check", false, "skip Ubuntu 22.04/24.04 guard for tests only")
		runSteps := cmd.Bool("run-steps", false, "run command steps even when --install-root is set")
		if err := cmd.Parse(args[1:]); err != nil {
			os.Exit(2)
		}
		runtime := mustLoadRuntime(*configPath)
		result, err := installer.ApplyLabInstall(runtime, installer.Options{
			ConfigPath:         *configPath,
			InstallRoot:        *installRoot,
			AgentBinary:        *agentBinary,
			CLIBinary:          *cliBinary,
			ProviderBinary:     *providerBinary,
			SystemdServicePath: *servicePath,
		}, installer.ApplyOptions{
			Ack:           *ack,
			EffectiveUID:  os.Geteuid(),
			OSReleasePath: *osRelease,
			SkipOSCheck:   *skipOSCheck,
			RunSteps:      *runSteps,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "install apply-lab failed: %v\n", err)
			os.Exit(1)
		}
		writeJSON(result)
	case "uninstall-plan":
		cmd := flag.NewFlagSet("kiro-cli install uninstall-plan", flag.ExitOnError)
		configPath := cmd.String("config", "configs/kiro.example.yaml", "path to kiro config")
		installRoot := cmd.String("install-root", "", "optional lab/staging root prefix")
		purge := cmd.Bool("purge", false, "include destructive config/state/log removal steps")
		if err := cmd.Parse(args[1:]); err != nil {
			os.Exit(2)
		}
		runtime := mustLoadRuntime(*configPath)
		plan, err := installer.BuildUninstallPlan(runtime, installer.Options{
			InstallRoot: *installRoot,
			Purge:       *purge,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "uninstall plan failed: %v\n", err)
			os.Exit(1)
		}
		writeJSON(plan)
	case "uninstall-apply-lab":
		cmd := flag.NewFlagSet("kiro-cli install uninstall-apply-lab", flag.ExitOnError)
		configPath := cmd.String("config", "configs/kiro.example.yaml", "path to kiro config")
		installRoot := cmd.String("install-root", "", "optional lab/staging root prefix; empty applies to real root")
		purge := cmd.Bool("purge", false, "include destructive config/state/log removal steps")
		ack := cmd.String("ack", "", "required value: KIRO_LAB_UNINSTALL_APPLY")
		osRelease := cmd.String("os-release", "/etc/os-release", "path to os-release for Ubuntu guard")
		skipOSCheck := cmd.Bool("skip-os-check", false, "skip Ubuntu 22.04/24.04 guard for tests only")
		runSteps := cmd.Bool("run-steps", false, "run command steps even when --install-root is set")
		if err := cmd.Parse(args[1:]); err != nil {
			os.Exit(2)
		}
		runtime := mustLoadRuntime(*configPath)
		result, err := installer.ApplyLabUninstall(runtime, installer.Options{
			InstallRoot: *installRoot,
			Purge:       *purge,
		}, installer.ApplyOptions{
			Ack:           *ack,
			EffectiveUID:  os.Geteuid(),
			OSReleasePath: *osRelease,
			SkipOSCheck:   *skipOSCheck,
			RunSteps:      *runSteps,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "install uninstall-apply-lab failed: %v\n", err)
			os.Exit(1)
		}
		writeJSON(result)
	default:
		usage()
	}
}

func runIncident(args []string) {
	if len(args) == 0 {
		usage()
	}
	switch args[0] {
	case "report":
		cmd := flag.NewFlagSet("kiro-cli incident report", flag.ExitOnError)
		configPath := cmd.String("config", "configs/kiro.example.yaml", "path to kiro config")
		outputDir := cmd.String("output-dir", "", "incident report output directory")
		incidentID := cmd.String("incident-id", "", "optional incident id")
		incidentType := cmd.String("type", "other", "incident type: attack, lost_ssh, update_failed, origin_ip_leaked, license_rebind, runtime_security, other")
		severity := cmd.String("severity", "medium", "incident severity")
		status := cmd.String("status", "open", "incident status")
		summary := cmd.String("summary", "", "incident summary")
		startedAt := cmd.String("started-at", "", "optional RFC3339 incident start time")
		detectedAt := cmd.String("detected-at", "", "optional RFC3339 detection time")
		supportBundleDir := cmd.String("support-bundle-dir", "", "optional support bundle directory")
		healthFile := cmd.String("health-file", "", "optional health report JSON file")
		alertsFile := cmd.String("alerts-file", "", "optional runtime alerts JSONL file")
		if err := cmd.Parse(args[1:]); err != nil {
			os.Exit(2)
		}
		runtime := mustLoadRuntime(*configPath)
		outDir := *outputDir
		if outDir == "" {
			outDir = runtime.Paths.StateDir + "/incidents"
		}
		result, err := support.BuildIncidentReport(support.IncidentOptions{
			OutputDir:        outDir,
			ConfigPath:       *configPath,
			Runtime:          runtime,
			IncidentID:       *incidentID,
			Type:             *incidentType,
			Severity:         *severity,
			Status:           *status,
			Summary:          *summary,
			StartedAt:        parseOptionalTime("started-at", *startedAt),
			DetectedAt:       parseOptionalTime("detected-at", *detectedAt),
			SupportBundleDir: *supportBundleDir,
			HealthPath:       *healthFile,
			AlertsPath:       *alertsFile,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "incident report failed: %v\n", err)
			os.Exit(1)
		}
		writeJSON(result)
	default:
		usage()
	}
}

func runPilot(args []string) {
	if len(args) == 0 {
		usage()
	}
	switch args[0] {
	case "report":
		cmd := flag.NewFlagSet("kiro-cli pilot report", flag.ExitOnError)
		configPath := cmd.String("config", "configs/kiro.example.yaml", "path to kiro config")
		outputDir := cmd.String("output-dir", "", "pilot report output directory")
		pilotID := cmd.String("pilot-id", "", "optional pilot id")
		serverCount := cmd.Int("server-count", 0, "number of pilot VPS/servers")
		startedAt := cmd.String("started-at", "", "pilot start time RFC3339")
		endedAt := cmd.String("ended-at", "", "pilot end time RFC3339")
		healthFile := cmd.String("health-file", "", "health report JSON evidence")
		benchmarkFile := cmd.String("benchmark-file", "", "benchmark report JSON evidence")
		benchmarkEvidenceFile := cmd.String("benchmark-evidence-file", "", "benchmark evidence JSON")
		incidentDir := cmd.String("incident-dir", "", "incident drill report directory")
		updateEvidenceFile := cmd.String("update-evidence-file", "", "update/rollback drill evidence file")
		revocationFile := cmd.String("revocation-file", "", "revocation sync evidence file")
		proxyEvidenceFile := cmd.String("proxy-evidence-file", "", "proxy/WAF/Bot drill evidence file")
		if err := cmd.Parse(args[1:]); err != nil {
			os.Exit(2)
		}
		runtime := mustLoadRuntime(*configPath)
		outDir := *outputDir
		if outDir == "" {
			outDir = runtime.Paths.StateDir + "/pilot-reports"
		}
		result, err := pilot.BuildReport(pilot.Options{
			OutputDir:             outDir,
			ConfigPath:            *configPath,
			Runtime:               runtime,
			PilotID:               *pilotID,
			ServerCount:           *serverCount,
			StartedAt:             parseOptionalTime("started-at", *startedAt),
			EndedAt:               parseOptionalTime("ended-at", *endedAt),
			HealthPath:            *healthFile,
			BenchmarkPath:         *benchmarkFile,
			BenchmarkEvidencePath: *benchmarkEvidenceFile,
			IncidentDir:           *incidentDir,
			UpdateEvidencePath:    *updateEvidenceFile,
			RevocationPath:        *revocationFile,
			ProxyEvidencePath:     *proxyEvidenceFile,
			Now:                   time.Now().UTC(),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "pilot report failed: %v\n", err)
			os.Exit(1)
		}
		writeJSON(result)
	default:
		usage()
	}
}

func mustLoadRuntime(path string) config.RuntimeConfig {
	runtime, err := config.LoadRuntimeFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "runtime config load failed: %v\n", err)
		os.Exit(1)
	}
	return runtime
}

func parseOptionalTime(name string, value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s must be RFC3339: %v\n", name, err)
		os.Exit(2)
	}
	return parsed
}

func writeJSON(value any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		fmt.Fprintf(os.Stderr, "json encode failed: %v\n", err)
		os.Exit(1)
	}
}

func runUpdate(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: kiro-cli update check|apply|rollback")
		os.Exit(2)
	}
	switch args[0] {
	case "check":
		cmd := flag.NewFlagSet("kiro-cli update check", flag.ExitOnError)
		masterURL := cmd.String("master-url", "", "master server URL (e.g. https://firewall.vpsgen.com)")
		component := cmd.String("component", "kiro-client-waf", "component name")
		channel := cmd.String("channel", "stable", "release channel")
		currentVersion := cmd.String("current-version", buildinfo.Version, "current version")
		if err := cmd.Parse(args[1:]); err != nil {
			os.Exit(2)
		}
		if *masterURL == "" {
			fmt.Fprintln(os.Stderr, "error: --master-url is required")
			os.Exit(2)
		}
		if err := update.Check(*masterURL, *component, *channel, *currentVersion); err != nil {
			fmt.Fprintf(os.Stderr, "update check failed: %v\n", err)
			os.Exit(1)
		}
	case "apply":
		cmd := flag.NewFlagSet("kiro-cli update apply", flag.ExitOnError)
		masterURL := cmd.String("master-url", "", "master server URL (e.g. https://firewall.vpsgen.com)")
		component := cmd.String("component", "kiro-client-waf", "component name")
		channel := cmd.String("channel", "stable", "release channel")
		currentVersion := cmd.String("current-version", buildinfo.Version, "current version")
		binaryPath := cmd.String("binary-path", "", "path to the binary to update")
		serviceName := cmd.String("service", "", "systemd service name to restart")
		if err := cmd.Parse(args[1:]); err != nil {
			os.Exit(2)
		}
		if *masterURL == "" {
			fmt.Fprintln(os.Stderr, "error: --master-url is required")
			os.Exit(2)
		}
		if *binaryPath == "" {
			fmt.Fprintln(os.Stderr, "error: --binary-path is required")
			os.Exit(2)
		}
		if *serviceName == "" {
			fmt.Fprintln(os.Stderr, "error: --service is required")
			os.Exit(2)
		}
		if err := update.Apply(*masterURL, *component, *channel, *currentVersion, *binaryPath, *serviceName); err != nil {
			fmt.Fprintf(os.Stderr, "update apply failed: %v\n", err)
			os.Exit(1)
		}
	case "rollback":
		cmd := flag.NewFlagSet("kiro-cli update rollback", flag.ExitOnError)
		binaryPath := cmd.String("binary-path", "", "path to the binary to rollback")
		serviceName := cmd.String("service", "", "systemd service name to restart")
		if err := cmd.Parse(args[1:]); err != nil {
			os.Exit(2)
		}
		if *binaryPath == "" {
			fmt.Fprintln(os.Stderr, "error: --binary-path is required")
			os.Exit(2)
		}
		if *serviceName == "" {
			fmt.Fprintln(os.Stderr, "error: --service is required")
			os.Exit(2)
		}
		if err := update.Rollback(*binaryPath, *serviceName); err != nil {
			fmt.Fprintf(os.Stderr, "update rollback failed: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: kiro-cli update check|apply|rollback")
		os.Exit(2)
	}
}

func runReport(args []string) {
	cmd := flag.NewFlagSet("kiro-cli report", flag.ExitOnError)
	configPath := cmd.String("config", "configs/kiro.example.yaml", "path to kiro config")
	if err := cmd.Parse(args); err != nil {
		os.Exit(2)
	}
	runtime := mustLoadRuntime(*configPath)
	report := diagnostics.BuildSystemReport(runtime, time.Now().UTC())
	writeJSON(report)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: kiro-cli version | status | health | preflight | mode show|set | install plan|stage-lab|apply-lab|uninstall-plan|uninstall-apply-lab | update check|apply|rollback | report | incident report | pilot report | license fingerprint [--salt ID]")
	os.Exit(2)
}
