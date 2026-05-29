package diagnostics

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"kiro_waf/internal/shared/config"
)

type PreflightOptions struct {
	OSReleasePath     string
	EffectiveUID      int
	CommandLookup     func(string) (string, error)
	WritableRoot      string
	SkipCommandChecks bool
	Now               time.Time
}

type PreflightReport struct {
	GeneratedAt string  `json:"generated_at"`
	Status      string  `json:"status"`
	Checks      []Check `json:"checks"`
}

type osRelease struct {
	ID        string
	VersionID string
	Pretty    string
}

func BuildPreflight(runtimeCfg config.RuntimeConfig, opts PreflightOptions) PreflightReport {
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}
	checks := []Check{
		goosCheck(),
		osReleaseCheck(firstNonEmpty(opts.OSReleasePath, "/etc/os-release")),
		rootCheck(opts.EffectiveUID),
		adminAllowlistCheck(runtimeCfg),
		stateDirCheck(runtimeCfg, opts.WritableRoot),
	}
	if !opts.SkipCommandChecks {
		lookup := opts.CommandLookup
		if lookup == nil {
			lookup = exec.LookPath
		}
		checks = append(checks, commandCheck(lookup, "nft"))
		if runtimeCfg.XDP.Enabled {
			checks = append(checks, commandCheck(lookup, "ip"))
			checks = append(checks, commandCheck(lookup, "bpftool"))
			checks = append(checks, commandCheck(lookup, "clang"))
			checks = append(checks, commandCheck(lookup, "llvm-objdump"))
		}
		if runtimeCfg.Mode == "full" {
			checks = append(checks, commandCheck(lookup, "nginx"))
		}
		checks = append(checks, commandCheck(lookup, "systemctl"))
	}
	return PreflightReport{
		GeneratedAt: opts.Now.Format(time.RFC3339),
		Status:      overallStatus(checks),
		Checks:      checks,
	}
}

func goosCheck() Check {
	if runtime.GOOS == "linux" {
		return Check{Name: "os", Status: "pass", Message: "running on linux"}
	}
	return Check{Name: "os", Status: "fail", Severity: "critical", Message: "kiro_waf targets Linux/Ubuntu hosts"}
}

func osReleaseCheck(path string) Check {
	release, err := readOSRelease(path)
	if err != nil {
		return Check{Name: "ubuntu_release", Status: "warn", Severity: "medium", Message: "cannot read os-release: " + err.Error()}
	}
	if release.ID == "ubuntu" && strings.HasPrefix(release.VersionID, "22.04") {
		return Check{Name: "ubuntu_release", Status: "pass", Message: "Ubuntu 22.04 LTS detected"}
	}
	if release.ID == "ubuntu" && strings.HasPrefix(release.VersionID, "24.04") {
		return Check{Name: "ubuntu_release", Status: "warn", Severity: "medium", Message: "Ubuntu 24.04 detected; supported roadmap target, verify lab before production"}
	}
	return Check{Name: "ubuntu_release", Status: "warn", Severity: "medium", Message: "expected Ubuntu 22.04 LTS, got " + release.Pretty}
}

func rootCheck(euid int) Check {
	if euid == 0 {
		return Check{Name: "root", Status: "pass", Message: "running with root privileges"}
	}
	return Check{Name: "root", Status: "warn", Severity: "medium", Message: "root is required for firewall apply and some runtime checks"}
}

func stateDirCheck(runtimeCfg config.RuntimeConfig, writableRoot string) Check {
	stateDir := runtimeCfg.Paths.StateDir
	if strings.TrimSpace(writableRoot) != "" {
		stateDir = filepath.Join(writableRoot, "kiro-state-check")
	}
	if strings.TrimSpace(stateDir) == "" {
		return Check{Name: "state_dir", Status: "fail", Severity: "high", Message: "state dir is empty"}
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return Check{Name: "state_dir", Status: "fail", Severity: "high", Message: "cannot create state dir: " + err.Error()}
	}
	probe := filepath.Join(stateDir, ".kiro-preflight")
	if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
		return Check{Name: "state_dir", Status: "fail", Severity: "high", Message: "state dir is not writable: " + err.Error()}
	}
	_ = os.Remove(probe)
	return Check{Name: "state_dir", Status: "pass", Message: "state dir is writable"}
}

func commandCheck(lookup func(string) (string, error), name string) Check {
	path, err := lookup(name)
	if err != nil {
		return Check{Name: "command_" + name, Status: "warn", Severity: "medium", Message: name + " not found in PATH"}
	}
	return Check{Name: "command_" + name, Status: "pass", Message: path}
}

func readOSRelease(path string) (osRelease, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return osRelease{}, err
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
	if values["ID"] == "" {
		return osRelease{}, errors.New("ID missing")
	}
	return osRelease{
		ID:        values["ID"],
		VersionID: values["VERSION_ID"],
		Pretty:    firstNonEmpty(values["PRETTY_NAME"], values["ID"]+" "+values["VERSION_ID"]),
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
