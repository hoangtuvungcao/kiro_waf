package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"kiro_waf/internal/shared/buildinfo"
	"kiro_waf/internal/shared/config"
	"kiro_waf/internal/shared/storage"
)

const (
	DefaultConfigDir     = "/etc/kiro"
	DefaultLogDir        = "/var/log/kiro"
	DefaultRunDir        = "/run/kiro"
	DefaultBinDir        = "/usr/local/bin"
	DefaultSystemdDir    = "/etc/systemd/system"
	DefaultServiceSource = "deployments/systemd/kiro-agent.service"
	DefaultServiceName   = "kiro-agent.service"
	LabInstallAck        = "KIRO_LAB_INSTALL_APPLY"
	LabUninstallAck      = "KIRO_LAB_UNINSTALL_APPLY"
)

type Options struct {
	ConfigPath         string
	InstallRoot        string
	AgentBinary        string
	CLIBinary          string
	ProviderBinary     string
	SystemdServicePath string
	Purge              bool
	Now                time.Time
}

type Layout struct {
	ConfigDir         string `json:"config_dir"`
	ConfigFile        string `json:"config_file"`
	StateDir          string `json:"state_dir"`
	LastGoodConfigDir string `json:"last_good_config_dir"`
	LogDir            string `json:"log_dir"`
	RunDir            string `json:"run_dir"`
	BinDir            string `json:"bin_dir"`
	AgentBinary       string `json:"agent_binary"`
	CLIBinary         string `json:"cli_binary"`
	ProviderBinary    string `json:"provider_binary"`
	SystemdDir        string `json:"systemd_dir"`
	SystemdService    string `json:"systemd_service"`
}

type Plan struct {
	GeneratedAt string   `json:"generated_at"`
	Version     string   `json:"version"`
	Action      string   `json:"action"`
	InstallRoot string   `json:"install_root,omitempty"`
	Mode        string   `json:"mode"`
	ProductPlan string   `json:"plan"`
	Layout      Layout   `json:"layout"`
	Steps       []Step   `json:"steps"`
	Warnings    []string `json:"warnings,omitempty"`
}

type Step struct {
	Name         string `json:"name"`
	Action       string `json:"action"`
	Source       string `json:"source,omitempty"`
	Target       string `json:"target,omitempty"`
	Command      string `json:"command,omitempty"`
	Mode         string `json:"mode,omitempty"`
	Rollback     string `json:"rollback,omitempty"`
	RequiresRoot bool   `json:"requires_root"`
	Creates      bool   `json:"creates,omitempty"`
	Overwrites   bool   `json:"overwrites,omitempty"`
	Destructive  bool   `json:"destructive,omitempty"`
}

type StageResult struct {
	GeneratedAt  string        `json:"generated_at"`
	Version      string        `json:"version"`
	InstallRoot  string        `json:"install_root"`
	ManifestPath string        `json:"manifest_path"`
	Files        []StagedFile  `json:"files"`
	Directories  []string      `json:"directories"`
	Skipped      []SkippedStep `json:"skipped"`
}

type StagedFile struct {
	Target     string `json:"target"`
	StagePath  string `json:"stage_path"`
	Mode       string `json:"mode"`
	SHA256     string `json:"sha256"`
	SizeBytes  int64  `json:"size_bytes"`
	SourcePath string `json:"source_path,omitempty"`
}

type SkippedStep struct {
	Name   string `json:"name"`
	Action string `json:"action"`
	Reason string `json:"reason"`
}

type CommandRunner interface {
	Run(command string) error
}

type ShellRunner struct{}

type ApplyOptions struct {
	Ack           string
	EffectiveUID  int
	OSReleasePath string
	SkipOSCheck   bool
	RunSteps      bool
	Runner        CommandRunner
	Now           time.Time
}

type ApplyResult struct {
	GeneratedAt  string        `json:"generated_at"`
	Version      string        `json:"version"`
	Action       string        `json:"action"`
	InstallRoot  string        `json:"install_root,omitempty"`
	ManifestPath string        `json:"manifest_path,omitempty"`
	Steps        []AppliedStep `json:"steps"`
	Warnings     []string      `json:"warnings,omitempty"`
}

type AppliedStep struct {
	Name      string `json:"name"`
	Action    string `json:"action"`
	Status    string `json:"status"`
	Target    string `json:"target,omitempty"`
	Command   string `json:"command,omitempty"`
	Message   string `json:"message,omitempty"`
	SHA256    string `json:"sha256,omitempty"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
}

func (ShellRunner) Run(command string) error {
	cmd := exec.Command("sh", "-c", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %s: %w: %s", command, err, string(out))
	}
	return nil
}

func BuildInstallPlan(runtimeCfg config.RuntimeConfig, opts Options) (Plan, error) {
	if strings.TrimSpace(opts.ConfigPath) == "" {
		return Plan{}, errors.New("config path is required")
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}
	layout := defaultLayout(runtimeCfg)
	serviceSource := firstNonEmpty(opts.SystemdServicePath, DefaultServiceSource)
	plan := Plan{
		GeneratedAt: opts.Now.Format(time.RFC3339),
		Version:     buildinfo.Version,
		Action:      "install",
		InstallRoot: strings.TrimSpace(opts.InstallRoot),
		Mode:        runtimeCfg.Mode,
		ProductPlan: runtimeCfg.Plan,
		Layout:      layout,
		Warnings:    missingBinaryWarnings(opts),
	}
	preflightCommand := "kiro-cli"
	if strings.TrimSpace(opts.CLIBinary) != "" {
		preflightCommand = shellArg(opts.CLIBinary)
	}
	plan.Steps = append(plan.Steps,
		Step{Name: "preflight", Action: "run", Command: preflightCommand + " preflight --config " + shellArg(opts.ConfigPath), RequiresRoot: false},
		mkdirStep("create_config_dir", layout.ConfigDir, "0755", opts),
		mkdirStep("create_state_dir", layout.StateDir, "0755", opts),
		mkdirStep("create_last_good_config_dir", layout.LastGoodConfigDir, "0755", opts),
		mkdirStep("create_log_dir", layout.LogDir, "0755", opts),
		mkdirStep("create_run_dir", layout.RunDir, "0755", opts),
		copyStep("install_config", opts.ConfigPath, layout.ConfigFile, "0640", opts, "restore previous /etc/kiro/kiro.yaml from backup or last-good config"),
		copyStep("install_systemd_service", serviceSource, layout.SystemdService, "0644", opts, "remove service file and run systemctl daemon-reload"),
	)
	if strings.TrimSpace(opts.AgentBinary) != "" {
		plan.Steps = append(plan.Steps, copyStep("install_agent_binary", opts.AgentBinary, layout.AgentBinary, "0755", opts, "restore previous kiro-agent binary"))
	}
	if strings.TrimSpace(opts.CLIBinary) != "" {
		plan.Steps = append(plan.Steps, copyStep("install_cli_binary", opts.CLIBinary, layout.CLIBinary, "0755", opts, "restore previous kiro-cli binary"))
	}
	if strings.TrimSpace(opts.ProviderBinary) != "" {
		plan.Steps = append(plan.Steps, copyStep("install_provider_binary", opts.ProviderBinary, layout.ProviderBinary, "0755", opts, "restore previous kiro-provider binary"))
	}
	plan.Steps = append(plan.Steps,
		Step{Name: "firewall_dry_run_snapshot", Action: "run", Command: "kiro-agent --config " + shellArg(layout.ConfigFile) + " --firewall-dry-run --firewall-snapshot-dir " + shellArg(layout.LastGoodConfigDir), RequiresRoot: false},
		Step{Name: "proxy_dry_run", Action: "run", Command: "kiro-agent --config " + shellArg(layout.ConfigFile) + " --proxy-dry-run", RequiresRoot: false},
		Step{Name: "systemd_daemon_reload", Action: "run", Command: "systemctl daemon-reload", RequiresRoot: requiresRoot("", opts)},
		Step{Name: "enable_agent_service", Action: "run", Command: "systemctl enable --now kiro-agent.service", Rollback: "systemctl disable --now kiro-agent.service", RequiresRoot: requiresRoot("", opts)},
	)
	return plan, nil
}

func BuildUninstallPlan(runtimeCfg config.RuntimeConfig, opts Options) (Plan, error) {
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}
	layout := defaultLayout(runtimeCfg)
	plan := Plan{
		GeneratedAt: opts.Now.Format(time.RFC3339),
		Version:     buildinfo.Version,
		Action:      "uninstall",
		InstallRoot: strings.TrimSpace(opts.InstallRoot),
		Mode:        runtimeCfg.Mode,
		ProductPlan: runtimeCfg.Plan,
		Layout:      layout,
	}
	plan.Steps = append(plan.Steps,
		Step{Name: "stop_agent_service", Action: "run", Command: "systemctl disable --now kiro-agent.service", RequiresRoot: requiresRoot("", opts)},
		removeStep("remove_systemd_service", layout.SystemdService, opts),
		Step{Name: "systemd_daemon_reload", Action: "run", Command: "systemctl daemon-reload", RequiresRoot: requiresRoot("", opts)},
		removeStep("remove_agent_binary", layout.AgentBinary, opts),
		removeStep("remove_cli_binary", layout.CLIBinary, opts),
		removeStep("remove_provider_binary", layout.ProviderBinary, opts),
	)
	if opts.Purge {
		plan.Steps = append(plan.Steps,
			removeStep("purge_config_dir", layout.ConfigDir, opts),
			removeStep("purge_state_dir", layout.StateDir, opts),
			removeStep("purge_log_dir", layout.LogDir, opts),
			removeStep("purge_run_dir", layout.RunDir, opts),
		)
	} else {
		plan.Warnings = append(plan.Warnings, "config, license, state, and logs are kept; pass purge when a destructive removal is intended")
	}
	return plan, nil
}

func StageLabInstall(runtimeCfg config.RuntimeConfig, opts Options) (StageResult, error) {
	if strings.TrimSpace(opts.InstallRoot) == "" {
		return StageResult{}, errors.New("install root is required for lab staging")
	}
	plan, err := BuildInstallPlan(runtimeCfg, opts)
	if err != nil {
		return StageResult{}, err
	}
	result := StageResult{
		GeneratedAt: plan.GeneratedAt,
		Version:     plan.Version,
		InstallRoot: filepath.Clean(opts.InstallRoot),
	}
	for _, step := range plan.Steps {
		switch step.Action {
		case "mkdir":
			stagePath, err := rootedPath(opts.InstallRoot, step.Target)
			if err != nil {
				return StageResult{}, err
			}
			mode, err := parseMode(step.Mode, 0o755)
			if err != nil {
				return StageResult{}, err
			}
			if err := os.MkdirAll(stagePath, mode); err != nil {
				return StageResult{}, err
			}
			result.Directories = append(result.Directories, stagePath)
		case "copy_file":
			file, err := copyToRoot(opts.InstallRoot, step.Source, step.Target, step.Mode)
			if err != nil {
				return StageResult{}, fmt.Errorf("%s: %w", step.Name, err)
			}
			result.Files = append(result.Files, file)
		default:
			result.Skipped = append(result.Skipped, SkippedStep{
				Name:   step.Name,
				Action: step.Action,
				Reason: "lab staging only writes files and directories",
			})
		}
	}
	manifestTarget := filepath.Join(plan.Layout.StateDir, "install-manifest.json")
	manifestPath, err := rootedPath(opts.InstallRoot, manifestTarget)
	if err != nil {
		return StageResult{}, err
	}
	result.ManifestPath = manifestPath
	if err := storage.WriteJSONAtomic(manifestPath, result); err != nil {
		return StageResult{}, err
	}
	return result, nil
}

func ApplyLabInstall(runtimeCfg config.RuntimeConfig, opts Options, apply ApplyOptions) (ApplyResult, error) {
	if err := validateApplyGuard(opts, apply, LabInstallAck); err != nil {
		return ApplyResult{}, err
	}
	if apply.Now.IsZero() {
		apply.Now = time.Now().UTC()
	}
	plan, err := BuildInstallPlan(runtimeCfg, opts)
	if err != nil {
		return ApplyResult{}, err
	}
	result := ApplyResult{
		GeneratedAt: apply.Now.Format(time.RFC3339),
		Version:     buildinfo.Version,
		Action:      "install_apply_lab",
		InstallRoot: strings.TrimSpace(opts.InstallRoot),
		Warnings:    append([]string(nil), plan.Warnings...),
	}
	runSteps := apply.RunSteps || strings.TrimSpace(opts.InstallRoot) == ""
	runner := apply.Runner
	if runner == nil {
		runner = ShellRunner{}
	}
	for _, step := range plan.Steps {
		applied, err := applyInstallStep(step, opts, runner, runSteps)
		result.Steps = append(result.Steps, applied)
		if err != nil {
			return result, fmt.Errorf("%s: %w", step.Name, err)
		}
	}
	manifestTarget := filepath.Join(plan.Layout.StateDir, "install-apply-manifest.json")
	manifestPath, err := resolvedTargetPath(opts, manifestTarget)
	if err != nil {
		return result, err
	}
	result.ManifestPath = manifestPath
	if err := storage.WriteJSONAtomic(manifestPath, result); err != nil {
		return result, err
	}
	return result, nil
}

func ApplyLabUninstall(runtimeCfg config.RuntimeConfig, opts Options, apply ApplyOptions) (ApplyResult, error) {
	if err := validateApplyGuard(opts, apply, LabUninstallAck); err != nil {
		return ApplyResult{}, err
	}
	if apply.Now.IsZero() {
		apply.Now = time.Now().UTC()
	}
	plan, err := BuildUninstallPlan(runtimeCfg, opts)
	if err != nil {
		return ApplyResult{}, err
	}
	result := ApplyResult{
		GeneratedAt: apply.Now.Format(time.RFC3339),
		Version:     buildinfo.Version,
		Action:      "uninstall_apply_lab",
		InstallRoot: strings.TrimSpace(opts.InstallRoot),
		Warnings:    append([]string(nil), plan.Warnings...),
	}
	runSteps := apply.RunSteps || strings.TrimSpace(opts.InstallRoot) == ""
	runner := apply.Runner
	if runner == nil {
		runner = ShellRunner{}
	}
	for _, step := range plan.Steps {
		applied, err := applyUninstallStep(step, opts, runner, runSteps)
		result.Steps = append(result.Steps, applied)
		if err != nil {
			return result, fmt.Errorf("%s: %w", step.Name, err)
		}
	}
	return result, nil
}

func defaultLayout(runtimeCfg config.RuntimeConfig) Layout {
	stateDir := firstNonEmpty(runtimeCfg.Paths.StateDir, "/var/lib/kiro")
	lastGood := firstNonEmpty(runtimeCfg.Paths.LastGoodConfigDir, filepath.Join(stateDir, "last-good-config"))
	return Layout{
		ConfigDir:         DefaultConfigDir,
		ConfigFile:        filepath.Join(DefaultConfigDir, "kiro.yaml"),
		StateDir:          stateDir,
		LastGoodConfigDir: lastGood,
		LogDir:            DefaultLogDir,
		RunDir:            DefaultRunDir,
		BinDir:            DefaultBinDir,
		AgentBinary:       filepath.Join(DefaultBinDir, "kiro-agent"),
		CLIBinary:         filepath.Join(DefaultBinDir, "kiro-cli"),
		ProviderBinary:    filepath.Join(DefaultBinDir, "kiro-provider"),
		SystemdDir:        DefaultSystemdDir,
		SystemdService:    filepath.Join(DefaultSystemdDir, DefaultServiceName),
	}
}

func validateApplyGuard(opts Options, apply ApplyOptions, wantAck string) error {
	if strings.TrimSpace(apply.Ack) != wantAck {
		return fmt.Errorf("lab apply requires ack %s", wantAck)
	}
	if strings.TrimSpace(opts.InstallRoot) == "" && apply.EffectiveUID != 0 {
		return errors.New("real-root lab apply requires root")
	}
	if apply.SkipOSCheck {
		return nil
	}
	return validateUbuntuRelease(firstNonEmpty(apply.OSReleasePath, "/etc/os-release"))
}

func validateUbuntuRelease(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read os-release: %w", err)
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[key] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	if values["ID"] != "ubuntu" {
		return fmt.Errorf("installer lab apply expects Ubuntu, got %q", values["ID"])
	}
	version := values["VERSION_ID"]
	if strings.HasPrefix(version, "22.04") || strings.HasPrefix(version, "24.04") {
		return nil
	}
	return fmt.Errorf("installer lab apply expects Ubuntu 22.04 or 24.04, got %q", version)
}

func applyInstallStep(step Step, opts Options, runner CommandRunner, runSteps bool) (AppliedStep, error) {
	applied := AppliedStep{Name: step.Name, Action: step.Action, Target: step.Target, Command: step.Command}
	switch step.Action {
	case "mkdir":
		target, err := resolvedTargetPath(opts, step.Target)
		if err != nil {
			return applied, err
		}
		mode, err := parseMode(step.Mode, 0o755)
		if err != nil {
			return applied, err
		}
		if err := os.MkdirAll(target, mode); err != nil {
			return applied, err
		}
		applied.Status = "applied"
		applied.Target = target
		return applied, nil
	case "copy_file":
		file, err := copyToResolvedTarget(opts, step.Source, step.Target, step.Mode)
		if err != nil {
			return applied, err
		}
		applied.Status = "applied"
		applied.Target = file.StagePath
		applied.SHA256 = file.SHA256
		applied.SizeBytes = file.SizeBytes
		return applied, nil
	case "run":
		if !runSteps {
			applied.Status = "skipped"
			applied.Message = "run step skipped for install-root lab apply"
			return applied, nil
		}
		if err := runner.Run(step.Command); err != nil {
			return applied, err
		}
		applied.Status = "applied"
		return applied, nil
	default:
		applied.Status = "skipped"
		applied.Message = "unsupported install step action"
		return applied, nil
	}
}

func applyUninstallStep(step Step, opts Options, runner CommandRunner, runSteps bool) (AppliedStep, error) {
	applied := AppliedStep{Name: step.Name, Action: step.Action, Target: step.Target, Command: step.Command}
	switch step.Action {
	case "remove":
		target, err := resolvedTargetPath(opts, step.Target)
		if err != nil {
			return applied, err
		}
		if err := os.RemoveAll(target); err != nil {
			return applied, err
		}
		applied.Status = "applied"
		applied.Target = target
		return applied, nil
	case "run":
		if !runSteps {
			applied.Status = "skipped"
			applied.Message = "run step skipped for install-root lab apply"
			return applied, nil
		}
		if err := runner.Run(step.Command); err != nil {
			return applied, err
		}
		applied.Status = "applied"
		return applied, nil
	default:
		applied.Status = "skipped"
		applied.Message = "unsupported uninstall step action"
		return applied, nil
	}
}

func copyToResolvedTarget(opts Options, source string, target string, modeText string) (StagedFile, error) {
	if strings.TrimSpace(opts.InstallRoot) != "" {
		return copyToRoot(opts.InstallRoot, source, target, modeText)
	}
	return copyToTarget(source, target, modeText)
}

func copyToTarget(source string, target string, modeText string) (StagedFile, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return StagedFile{}, errors.New("source path is empty")
	}
	if !filepath.IsAbs(target) {
		return StagedFile{}, fmt.Errorf("target must be absolute: %s", target)
	}
	mode, err := parseMode(modeText, 0o644)
	if err != nil {
		return StagedFile{}, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return StagedFile{}, err
	}
	src, err := os.Open(source)
	if err != nil {
		return StagedFile{}, err
	}
	defer src.Close()
	dst, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return StagedFile{}, err
	}
	hasher := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(dst, hasher), src)
	closeErr := dst.Close()
	if copyErr != nil {
		return StagedFile{}, copyErr
	}
	if closeErr != nil {
		return StagedFile{}, closeErr
	}
	if err := os.Chmod(target, mode); err != nil {
		return StagedFile{}, err
	}
	return StagedFile{
		Target:     filepath.Clean(target),
		StagePath:  filepath.Clean(target),
		Mode:       modeString(mode),
		SHA256:     hex.EncodeToString(hasher.Sum(nil)),
		SizeBytes:  size,
		SourcePath: source,
	}, nil
}

func resolvedTargetPath(opts Options, target string) (string, error) {
	if strings.TrimSpace(opts.InstallRoot) != "" {
		return rootedPath(opts.InstallRoot, target)
	}
	if !filepath.IsAbs(target) {
		return "", fmt.Errorf("target must be absolute: %s", target)
	}
	return filepath.Clean(target), nil
}

func mkdirStep(name string, target string, mode string, opts Options) Step {
	return Step{Name: name, Action: "mkdir", Target: target, Mode: mode, RequiresRoot: requiresRoot(target, opts), Creates: true}
}

func copyStep(name string, source string, target string, mode string, opts Options, rollback string) Step {
	return Step{Name: name, Action: "copy_file", Source: source, Target: target, Mode: mode, Rollback: rollback, RequiresRoot: requiresRoot(target, opts), Creates: true, Overwrites: true}
}

func removeStep(name string, target string, opts Options) Step {
	return Step{Name: name, Action: "remove", Target: target, RequiresRoot: requiresRoot(target, opts), Destructive: true}
}

func requiresRoot(target string, opts Options) bool {
	if strings.TrimSpace(opts.InstallRoot) != "" {
		return false
	}
	if target == "" {
		return true
	}
	return strings.HasPrefix(filepath.Clean(target), "/etc/") ||
		strings.HasPrefix(filepath.Clean(target), "/usr/") ||
		strings.HasPrefix(filepath.Clean(target), "/var/") ||
		strings.HasPrefix(filepath.Clean(target), "/run/")
}

func missingBinaryWarnings(opts Options) []string {
	var warnings []string
	if strings.TrimSpace(opts.AgentBinary) == "" {
		warnings = append(warnings, "agent binary source is empty; install plan will not copy kiro-agent")
	}
	if strings.TrimSpace(opts.CLIBinary) == "" {
		warnings = append(warnings, "cli binary source is empty; install plan will not copy kiro-cli")
	}
	return warnings
}

func copyToRoot(root string, source string, target string, modeText string) (StagedFile, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return StagedFile{}, errors.New("source path is empty")
	}
	stagePath, err := rootedPath(root, target)
	if err != nil {
		return StagedFile{}, err
	}
	mode, err := parseMode(modeText, 0o644)
	if err != nil {
		return StagedFile{}, err
	}
	if err := os.MkdirAll(filepath.Dir(stagePath), 0o755); err != nil {
		return StagedFile{}, err
	}
	src, err := os.Open(source)
	if err != nil {
		return StagedFile{}, err
	}
	defer src.Close()
	dst, err := os.OpenFile(stagePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return StagedFile{}, err
	}
	hasher := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(dst, hasher), src)
	closeErr := dst.Close()
	if copyErr != nil {
		return StagedFile{}, copyErr
	}
	if closeErr != nil {
		return StagedFile{}, closeErr
	}
	if err := os.Chmod(stagePath, mode); err != nil {
		return StagedFile{}, err
	}
	return StagedFile{
		Target:     filepath.Clean(target),
		StagePath:  stagePath,
		Mode:       modeString(mode),
		SHA256:     hex.EncodeToString(hasher.Sum(nil)),
		SizeBytes:  size,
		SourcePath: source,
	}, nil
}

func rootedPath(root string, target string) (string, error) {
	root = filepath.Clean(strings.TrimSpace(root))
	if root == "." || root == string(os.PathSeparator) || root == "" {
		return "", errors.New("unsafe install root")
	}
	target = filepath.Clean(target)
	if !filepath.IsAbs(target) {
		return "", fmt.Errorf("target must be absolute: %s", target)
	}
	rel := strings.TrimPrefix(target, string(os.PathSeparator))
	stagePath := filepath.Join(root, rel)
	cleanRoot := filepath.Clean(root)
	if stagePath != cleanRoot && !strings.HasPrefix(stagePath, cleanRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("target escapes install root: %s", target)
	}
	return stagePath, nil
}

func parseMode(value string, fallback os.FileMode) (os.FileMode, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseUint(value, 8, 32)
	if err != nil {
		return 0, err
	}
	return os.FileMode(parsed), nil
}

func modeString(mode os.FileMode) string {
	return fmt.Sprintf("%04o", mode.Perm())
}

func shellArg(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
