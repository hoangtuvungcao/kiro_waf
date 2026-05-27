package firewall

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"kiro_waf/internal/shared/config"
	"kiro_waf/internal/shared/storage"
)

const pendingApplyFile = "pending-firewall-apply.json"

type Runner interface {
	Check(ruleset string) error
	Apply(ruleset string) error
	ListRuleset() (string, error)
}

type NFTRunner struct {
	Binary string
}

type ApplyOptions struct {
	StateDir        string
	SnapshotDir     string
	RollbackSeconds int
	Now             time.Time
}

type ApplyResult struct {
	AppliedRulesetSHA256 string
	SnapshotPath         string
	PendingPath          string
	RollbackDeadline     string
}

type PendingApply struct {
	CreatedAt        string `json:"created_at"`
	RollbackDeadline string `json:"rollback_deadline"`
	PreviousRuleset  string `json:"previous_ruleset"`
	PreviousSHA256   string `json:"previous_sha256"`
	AppliedSHA256    string `json:"applied_sha256"`
	Mode             string `json:"mode"`
	Confirmed        bool   `json:"confirmed"`
}

func (r NFTRunner) Check(ruleset string) error {
	return r.run([]string{"-c", "-f", "-"}, ruleset)
}

func (r NFTRunner) Apply(ruleset string) error {
	return r.run([]string{"-f", "-"}, ruleset)
}

func (r NFTRunner) ListRuleset() (string, error) {
	binary := r.Binary
	if binary == "" {
		binary = "nft"
	}
	cmd := exec.Command(binary, "list", "ruleset")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("nft list ruleset failed: %w: %s", err, string(out))
	}
	return string(out), nil
}

func (r NFTRunner) run(args []string, stdin string) error {
	binary := r.Binary
	if binary == "" {
		binary = "nft"
	}
	cmd := exec.Command(binary, args...)
	cmd.Stdin = strings.NewReader(stdin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nft %s failed: %w: %s", strings.Join(args, " "), err, stderr.String())
	}
	return nil
}

func ApplyNftables(runtime config.RuntimeConfig, plan Plan, runner Runner, opts ApplyOptions) (ApplyResult, error) {
	if runner == nil {
		return ApplyResult{}, errors.New("firewall runner is required")
	}
	if strings.TrimSpace(plan.Ruleset) == "" || strings.TrimSpace(plan.SHA256) == "" {
		return ApplyResult{}, errors.New("generated firewall plan is required")
	}
	if runtime.Firewall.SSHAdminOnly && len(firstNonEmptySlice(runtime.Firewall.AdminCIDRs, runtime.AdminCIDRs)) == 0 {
		return ApplyResult{}, errors.New("admin CIDR allowlist is required before firewall apply")
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}
	if opts.RollbackSeconds <= 0 {
		opts.RollbackSeconds = 60
	}
	if opts.StateDir == "" {
		opts.StateDir = runtime.Paths.StateDir
	}
	if opts.SnapshotDir == "" {
		opts.SnapshotDir = runtime.Paths.LastGoodConfigDir
	}
	if opts.StateDir == "" || opts.SnapshotDir == "" {
		return ApplyResult{}, errors.New("state and snapshot directories are required")
	}

	if err := runner.Check(plan.Ruleset); err != nil {
		return ApplyResult{}, err
	}
	previousRuleset, err := runner.ListRuleset()
	if err != nil {
		return ApplyResult{}, err
	}
	previousPlan := Plan{Ruleset: previousRuleset, SHA256: hashRuleset(previousRuleset)}
	snapshotPath, err := WriteLastGoodSnapshot(opts.SnapshotDir, runtime, previousPlan, opts.Now)
	if err != nil {
		return ApplyResult{}, err
	}
	deadline := opts.Now.Add(time.Duration(opts.RollbackSeconds) * time.Second)
	pending := PendingApply{
		CreatedAt:        opts.Now.Format(time.RFC3339),
		RollbackDeadline: deadline.Format(time.RFC3339),
		PreviousRuleset:  previousRuleset,
		PreviousSHA256:   previousPlan.SHA256,
		AppliedSHA256:    plan.SHA256,
		Mode:             runtime.Mode,
		Confirmed:        false,
	}
	pendingPath := PendingApplyPath(opts.StateDir)
	if err := storage.WriteJSONAtomic(pendingPath, pending); err != nil {
		return ApplyResult{}, err
	}
	if err := runner.Apply(plan.Ruleset); err != nil {
		_ = runner.Apply(previousRuleset)
		_ = removePending(pendingPath)
		return ApplyResult{}, err
	}

	return ApplyResult{
		AppliedRulesetSHA256: plan.SHA256,
		SnapshotPath:         snapshotPath,
		PendingPath:          pendingPath,
		RollbackDeadline:     pending.RollbackDeadline,
	}, nil
}

func ConfirmPendingApply(stateDir string) error {
	if strings.TrimSpace(stateDir) == "" {
		return errors.New("state directory is required")
	}
	return removePending(PendingApplyPath(stateDir))
}

func RollbackPendingApply(stateDir string, runner Runner) error {
	if runner == nil {
		return errors.New("firewall runner is required")
	}
	pending, err := LoadPendingApply(stateDir)
	if err != nil {
		return err
	}
	if err := runner.Apply(pending.PreviousRuleset); err != nil {
		return err
	}
	return removePending(PendingApplyPath(stateDir))
}

func RollbackIfExpired(stateDir string, runner Runner, now time.Time) (bool, error) {
	pending, err := LoadPendingApply(stateDir)
	if err != nil {
		return false, err
	}
	deadline, err := time.Parse(time.RFC3339, pending.RollbackDeadline)
	if err != nil {
		return false, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if now.Before(deadline) {
		return false, nil
	}
	return true, RollbackPendingApply(stateDir, runner)
}

func LoadPendingApply(stateDir string) (PendingApply, error) {
	if strings.TrimSpace(stateDir) == "" {
		return PendingApply{}, errors.New("state directory is required")
	}
	var pending PendingApply
	if err := storage.ReadJSON(PendingApplyPath(stateDir), &pending); err != nil {
		return PendingApply{}, err
	}
	if pending.PreviousRuleset == "" {
		return PendingApply{}, errors.New("pending rollback has empty previous ruleset")
	}
	return pending, nil
}

func PendingApplyPath(stateDir string) string {
	return filepath.Join(stateDir, pendingApplyFile)
}

func removePending(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func hashRuleset(ruleset string) string {
	sum := sha256.Sum256([]byte(ruleset))
	return fmt.Sprintf("%x", sum[:])
}
