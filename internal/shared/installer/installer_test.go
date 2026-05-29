package installer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kiro_waf/internal/shared/config"
)

func TestBuildInstallPlanIncludesSafetySteps(t *testing.T) {
	plan, err := BuildInstallPlan(testRuntime(), Options{
		ConfigPath: "/tmp/kiro.yaml",
		Now:        time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("build install plan: %v", err)
	}
	if plan.GeneratedAt != "2026-05-28T00:00:00Z" {
		t.Fatalf("generated at = %s", plan.GeneratedAt)
	}
	assertStep(t, plan, "preflight", "run")
	assertStep(t, plan, "firewall_dry_run_snapshot", "run")
	assertStep(t, plan, "enable_agent_service", "run")
	if !contains(plan.Warnings, "agent binary source is empty") {
		t.Fatalf("expected missing agent binary warning, got %#v", plan.Warnings)
	}
	for _, step := range plan.Steps {
		if step.Target == "/etc/kiro" && !step.RequiresRoot {
			t.Fatalf("real install /etc target should require root")
		}
	}
}

func TestStageLabInstallWritesFilesUnderRoot(t *testing.T) {
	root := t.TempDir()
	srcDir := t.TempDir()
	configPath := filepath.Join(srcDir, "kiro.yaml")
	agentPath := filepath.Join(srcDir, "kiro-agent")
	cliPath := filepath.Join(srcDir, "kiro-cli")
	servicePath := filepath.Join(srcDir, "kiro-agent.service")
	writeFile(t, configPath, "mode: server\n", 0o644)
	writeFile(t, agentPath, "agent", 0o755)
	writeFile(t, cliPath, "cli", 0o755)
	writeFile(t, servicePath, "[Service]\nExecStart=/usr/local/bin/kiro-agent\n", 0o644)

	result, err := StageLabInstall(testRuntime(), Options{
		ConfigPath:         configPath,
		InstallRoot:        root,
		AgentBinary:        agentPath,
		CLIBinary:          cliPath,
		SystemdServicePath: servicePath,
		Now:                time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("stage lab install: %v", err)
	}
	assertPathExists(t, filepath.Join(root, "etc/kiro/kiro.yaml"))
	assertPathExists(t, filepath.Join(root, "usr/local/bin/kiro-agent"))
	assertPathExists(t, filepath.Join(root, "usr/local/bin/kiro-cli"))
	assertPathExists(t, filepath.Join(root, "etc/systemd/system/kiro-agent.service"))
	assertPathExists(t, result.ManifestPath)
	if len(result.Files) < 4 {
		t.Fatalf("expected staged files, got %#v", result.Files)
	}
	for _, file := range result.Files {
		if !strings.HasPrefix(file.StagePath, root+string(os.PathSeparator)) {
			t.Fatalf("stage file escaped root: %#v", file)
		}
		if file.SHA256 == "" || file.SizeBytes <= 0 {
			t.Fatalf("missing checksum/size: %#v", file)
		}
	}
}

func TestStageLabInstallRequiresRootPrefix(t *testing.T) {
	_, err := StageLabInstall(testRuntime(), Options{ConfigPath: "/tmp/kiro.yaml"})
	if err == nil {
		t.Fatal("expected missing install root error")
	}
}

func TestBuildUninstallPlanKeepsDataUnlessPurge(t *testing.T) {
	plan, err := BuildUninstallPlan(testRuntime(), Options{})
	if err != nil {
		t.Fatalf("build uninstall plan: %v", err)
	}
	assertStep(t, plan, "remove_systemd_service", "remove")
	if hasStep(plan, "purge_state_dir") {
		t.Fatalf("state dir should not be purged by default")
	}
	purge, err := BuildUninstallPlan(testRuntime(), Options{Purge: true})
	if err != nil {
		t.Fatalf("build purge uninstall plan: %v", err)
	}
	assertStep(t, purge, "purge_state_dir", "remove")
}

func TestApplyLabInstallWritesFilesAndRunsCommandsWhenEnabled(t *testing.T) {
	root := t.TempDir()
	srcDir := t.TempDir()
	configPath := filepath.Join(srcDir, "kiro.yaml")
	agentPath := filepath.Join(srcDir, "kiro-agent")
	cliPath := filepath.Join(srcDir, "kiro-cli")
	servicePath := filepath.Join(srcDir, "kiro-agent.service")
	writeFile(t, configPath, "mode: server\n", 0o644)
	writeFile(t, agentPath, "agent", 0o755)
	writeFile(t, cliPath, "cli", 0o755)
	writeFile(t, servicePath, "[Service]\nExecStart=/usr/local/bin/kiro-agent\n", 0o644)

	runner := &fakeCommandRunner{}
	result, err := ApplyLabInstall(testRuntime(), Options{
		ConfigPath:         configPath,
		InstallRoot:        root,
		AgentBinary:        agentPath,
		CLIBinary:          cliPath,
		SystemdServicePath: servicePath,
		Now:                time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC),
	}, ApplyOptions{
		Ack:          LabInstallAck,
		EffectiveUID: 1000,
		SkipOSCheck:  true,
		RunSteps:     true,
		Runner:       runner,
		Now:          time.Date(2026, 5, 28, 1, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("apply lab install: %v", err)
	}
	if result.Action != "install_apply_lab" || result.ManifestPath == "" {
		t.Fatalf("unexpected result: %#v", result)
	}
	assertPathExists(t, filepath.Join(root, "etc/kiro/kiro.yaml"))
	assertPathExists(t, filepath.Join(root, "usr/local/bin/kiro-agent"))
	assertPathExists(t, filepath.Join(root, "usr/local/bin/kiro-cli"))
	assertPathExists(t, filepath.Join(root, "var/lib/kiro/install-apply-manifest.json"))
	if len(runner.commands) != 5 {
		t.Fatalf("commands = %#v, want preflight/firewall/proxy/systemd/service", runner.commands)
	}
	if !strings.Contains(runner.commands[0], cliPath) || !strings.Contains(runner.commands[0], "preflight") {
		t.Fatalf("preflight should use source cli binary, commands=%#v", runner.commands)
	}
}

func TestApplyLabInstallSkipsRunStepsForInstallRootByDefault(t *testing.T) {
	root := t.TempDir()
	srcDir := t.TempDir()
	configPath := filepath.Join(srcDir, "kiro.yaml")
	servicePath := filepath.Join(srcDir, "kiro-agent.service")
	writeFile(t, configPath, "mode: server\n", 0o644)
	writeFile(t, servicePath, "[Service]\nExecStart=/usr/local/bin/kiro-agent\n", 0o644)
	runner := &fakeCommandRunner{}

	result, err := ApplyLabInstall(testRuntime(), Options{
		ConfigPath:         configPath,
		InstallRoot:        root,
		SystemdServicePath: servicePath,
	}, ApplyOptions{
		Ack:          LabInstallAck,
		EffectiveUID: 1000,
		SkipOSCheck:  true,
		Runner:       runner,
	})
	if err != nil {
		t.Fatalf("apply lab install: %v", err)
	}
	if len(runner.commands) != 0 {
		t.Fatalf("run steps should be skipped, got %#v", runner.commands)
	}
	if !hasAppliedStatus(result.Steps, "preflight", "skipped") {
		t.Fatalf("preflight should be skipped in install-root apply: %#v", result.Steps)
	}
}

func TestApplyLabInstallRequiresAck(t *testing.T) {
	_, err := ApplyLabInstall(testRuntime(), Options{ConfigPath: "/tmp/kiro.yaml", InstallRoot: t.TempDir()}, ApplyOptions{
		EffectiveUID: 1000,
		SkipOSCheck:  true,
	})
	if err == nil || !strings.Contains(err.Error(), LabInstallAck) {
		t.Fatalf("expected ack error, got %v", err)
	}
}

func TestApplyLabInstallRequiresRootForRealRoot(t *testing.T) {
	_, err := ApplyLabInstall(testRuntime(), Options{ConfigPath: "/tmp/kiro.yaml"}, ApplyOptions{
		Ack:          LabInstallAck,
		EffectiveUID: 1000,
		SkipOSCheck:  true,
	})
	if err == nil || !strings.Contains(err.Error(), "requires root") {
		t.Fatalf("expected root error, got %v", err)
	}
}

func TestApplyLabInstallChecksUbuntuRelease(t *testing.T) {
	osRelease := filepath.Join(t.TempDir(), "os-release")
	writeFile(t, osRelease, "ID=debian\nVERSION_ID=\"12\"\n", 0o644)
	_, err := ApplyLabInstall(testRuntime(), Options{ConfigPath: "/tmp/kiro.yaml", InstallRoot: t.TempDir()}, ApplyOptions{
		Ack:           LabInstallAck,
		EffectiveUID:  1000,
		OSReleasePath: osRelease,
	})
	if err == nil || !strings.Contains(err.Error(), "expects Ubuntu") {
		t.Fatalf("expected Ubuntu guard error, got %v", err)
	}
}

func TestApplyLabUninstallRemovesFilesAndKeepsStateByDefault(t *testing.T) {
	root := t.TempDir()
	createInstalledTree(t, root)
	result, err := ApplyLabUninstall(testRuntime(), Options{
		InstallRoot: root,
	}, ApplyOptions{
		Ack:          LabUninstallAck,
		EffectiveUID: 1000,
		SkipOSCheck:  true,
	})
	if err != nil {
		t.Fatalf("apply lab uninstall: %v", err)
	}
	if result.Action != "uninstall_apply_lab" {
		t.Fatalf("unexpected result: %#v", result)
	}
	assertPathMissing(t, filepath.Join(root, "usr/local/bin/kiro-agent"))
	assertPathMissing(t, filepath.Join(root, "usr/local/bin/kiro-cli"))
	assertPathMissing(t, filepath.Join(root, "etc/systemd/system/kiro-agent.service"))
	assertPathExists(t, filepath.Join(root, "var/lib/kiro/state.json"))
	assertPathExists(t, filepath.Join(root, "etc/kiro/kiro.yaml"))
}

func TestApplyLabUninstallPurgeRemovesConfigStateAndLogs(t *testing.T) {
	root := t.TempDir()
	createInstalledTree(t, root)
	if _, err := ApplyLabUninstall(testRuntime(), Options{
		InstallRoot: root,
		Purge:       true,
	}, ApplyOptions{
		Ack:          LabUninstallAck,
		EffectiveUID: 1000,
		SkipOSCheck:  true,
	}); err != nil {
		t.Fatalf("apply lab uninstall purge: %v", err)
	}
	assertPathMissing(t, filepath.Join(root, "etc/kiro"))
	assertPathMissing(t, filepath.Join(root, "var/lib/kiro"))
	assertPathMissing(t, filepath.Join(root, "var/log/kiro"))
	assertPathMissing(t, filepath.Join(root, "run/kiro"))
}

func TestApplyLabInstallReturnsRunnerError(t *testing.T) {
	root := t.TempDir()
	srcDir := t.TempDir()
	configPath := filepath.Join(srcDir, "kiro.yaml")
	servicePath := filepath.Join(srcDir, "kiro-agent.service")
	writeFile(t, configPath, "mode: server\n", 0o644)
	writeFile(t, servicePath, "[Service]\nExecStart=/usr/local/bin/kiro-agent\n", 0o644)
	runnerErr := errors.New("preflight failed")
	_, err := ApplyLabInstall(testRuntime(), Options{
		ConfigPath:         configPath,
		InstallRoot:        root,
		SystemdServicePath: servicePath,
	}, ApplyOptions{
		Ack:          LabInstallAck,
		EffectiveUID: 1000,
		SkipOSCheck:  true,
		RunSteps:     true,
		Runner:       &fakeCommandRunner{err: runnerErr},
	})
	if !errors.Is(err, runnerErr) {
		t.Fatalf("error = %v, want runner error", err)
	}
}

func assertStep(t *testing.T, plan Plan, name string, action string) {
	t.Helper()
	for _, step := range plan.Steps {
		if step.Name == name {
			if step.Action != action {
				t.Fatalf("step %s action = %s, want %s", name, step.Action, action)
			}
			return
		}
	}
	t.Fatalf("step %s not found in %#v", name, plan.Steps)
}

func hasAppliedStatus(steps []AppliedStep, name string, status string) bool {
	for _, step := range steps {
		if step.Name == name && step.Status == status {
			return true
		}
	}
	return false
}

func hasStep(plan Plan, name string) bool {
	for _, step := range plan.Steps {
		if step.Name == name {
			return true
		}
	}
	return false
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func writeFile(t *testing.T, path string, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for file %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s missing, got %v", path, err)
	}
}

func createInstalledTree(t *testing.T, root string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "etc/kiro/kiro.yaml"), "mode: server\n", 0o640)
	writeFile(t, filepath.Join(root, "var/lib/kiro/state.json"), "{}", 0o644)
	writeFile(t, filepath.Join(root, "var/log/kiro/agent.log"), "log\n", 0o644)
	writeFile(t, filepath.Join(root, "run/kiro/pid"), "1\n", 0o644)
	writeFile(t, filepath.Join(root, "usr/local/bin/kiro-agent"), "agent", 0o755)
	writeFile(t, filepath.Join(root, "usr/local/bin/kiro-cli"), "cli", 0o755)
	writeFile(t, filepath.Join(root, "usr/local/bin/kiro-provider"), "provider", 0o755)
	writeFile(t, filepath.Join(root, "etc/systemd/system/kiro-agent.service"), "[Service]\n", 0o644)
}

type fakeCommandRunner struct {
	commands []string
	err      error
}

func (r *fakeCommandRunner) Run(command string) error {
	r.commands = append(r.commands, command)
	return r.err
}

func testRuntime() config.RuntimeConfig {
	return config.RuntimeConfig{
		Mode: "full",
		Plan: "school_smb",
		Paths: config.RuntimePaths{
			StateDir:          "/var/lib/kiro",
			LastGoodConfigDir: "/var/lib/kiro/last-good-config",
		},
		Safety: config.RuntimeSafety{
			RollbackTimerSeconds: 60,
		},
	}
}
