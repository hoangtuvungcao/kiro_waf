package firewall

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kiro_waf/internal/shared/storage"
)

func TestApplyNftablesCreatesPendingRollback(t *testing.T) {
	runtime := serverRuntime()
	dir := t.TempDir()
	runtime.Paths.StateDir = filepath.Join(dir, "state")
	runtime.Paths.LastGoodConfigDir = filepath.Join(dir, "last-good")
	plan, err := GenerateNftables(runtime, Options{})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	runner := &fakeRunner{current: "table inet old {}\n"}

	result, err := ApplyNftables(runtime, plan, runner, ApplyOptions{
		Now:             time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC),
		RollbackSeconds: 30,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if result.AppliedRulesetSHA256 != plan.SHA256 {
		t.Fatal("expected applied hash in result")
	}
	if runner.current != plan.Ruleset {
		t.Fatal("expected runner to hold applied ruleset")
	}
	if runner.checks != 1 || runner.applies != 1 || runner.lists != 1 {
		t.Fatalf("unexpected runner calls: %#v", runner)
	}
	if _, err := os.Stat(result.SnapshotPath); err != nil {
		t.Fatalf("snapshot missing: %v", err)
	}
	var pending PendingApply
	if err := storage.ReadJSON(result.PendingPath, &pending); err != nil {
		t.Fatalf("read pending: %v", err)
	}
	if pending.PreviousRuleset != "table inet old {}\n" || pending.AppliedSHA256 != plan.SHA256 {
		t.Fatal("pending rollback did not record previous/applied rulesets")
	}
	if pending.RollbackDeadline != "2026-05-28T00:00:30Z" {
		t.Fatalf("deadline = %s", pending.RollbackDeadline)
	}
}

func TestApplyNftablesRollsBackWhenApplyFails(t *testing.T) {
	runtime := serverRuntime()
	dir := t.TempDir()
	runtime.Paths.StateDir = filepath.Join(dir, "state")
	runtime.Paths.LastGoodConfigDir = filepath.Join(dir, "last-good")
	plan, err := GenerateNftables(runtime, Options{})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	runner := &fakeRunner{current: "previous", applyErr: errors.New("boom")}

	if _, err := ApplyNftables(runtime, plan, runner, ApplyOptions{}); err == nil {
		t.Fatal("expected apply error")
	}
	if runner.current != "previous" {
		t.Fatal("expected rollback to previous ruleset after apply failure")
	}
	if runner.applies != 2 {
		t.Fatalf("applies = %d, want apply + rollback", runner.applies)
	}
}

func TestConfirmPendingApplyRemovesPendingFile(t *testing.T) {
	runtime := serverRuntime()
	dir := t.TempDir()
	runtime.Paths.StateDir = filepath.Join(dir, "state")
	runtime.Paths.LastGoodConfigDir = filepath.Join(dir, "last-good")
	plan, err := GenerateNftables(runtime, Options{})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if _, err := ApplyNftables(runtime, plan, &fakeRunner{current: "previous"}, ApplyOptions{}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := ConfirmPendingApply(runtime.Paths.StateDir); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if _, err := os.Stat(PendingApplyPath(runtime.Paths.StateDir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected pending file removed, got %v", err)
	}
}

func TestRollbackIfExpiredAppliesPreviousRuleset(t *testing.T) {
	runtime := serverRuntime()
	dir := t.TempDir()
	runtime.Paths.StateDir = filepath.Join(dir, "state")
	runtime.Paths.LastGoodConfigDir = filepath.Join(dir, "last-good")
	plan, err := GenerateNftables(runtime, Options{})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	now := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	runner := &fakeRunner{current: "previous"}
	if _, err := ApplyNftables(runtime, plan, runner, ApplyOptions{Now: now, RollbackSeconds: 10}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	rolledBack, err := RollbackIfExpired(runtime.Paths.StateDir, runner, now.Add(11*time.Second))
	if err != nil {
		t.Fatalf("rollback if expired: %v", err)
	}
	if !rolledBack {
		t.Fatal("expected expired pending apply to rollback")
	}
	if runner.current != "previous" {
		t.Fatal("expected previous ruleset restored")
	}
}

func TestRollbackIfExpiredBeforeDeadlineDoesNothing(t *testing.T) {
	runtime := serverRuntime()
	dir := t.TempDir()
	runtime.Paths.StateDir = filepath.Join(dir, "state")
	runtime.Paths.LastGoodConfigDir = filepath.Join(dir, "last-good")
	plan, err := GenerateNftables(runtime, Options{})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	now := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	runner := &fakeRunner{current: "previous"}
	if _, err := ApplyNftables(runtime, plan, runner, ApplyOptions{Now: now, RollbackSeconds: 10}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	rolledBack, err := RollbackIfExpired(runtime.Paths.StateDir, runner, now.Add(5*time.Second))
	if err != nil {
		t.Fatalf("rollback if expired: %v", err)
	}
	if rolledBack {
		t.Fatal("expected no rollback before deadline")
	}
	if runner.current != plan.Ruleset {
		t.Fatal("expected applied ruleset to remain before deadline")
	}
}

type fakeRunner struct {
	current  string
	checkErr error
	applyErr error
	listErr  error
	checks   int
	applies  int
	lists    int
}

func (r *fakeRunner) Check(_ string) error {
	r.checks++
	return r.checkErr
}

func (r *fakeRunner) Apply(ruleset string) error {
	r.applies++
	if r.applyErr != nil {
		err := r.applyErr
		r.applyErr = nil
		return err
	}
	r.current = ruleset
	return nil
}

func (r *fakeRunner) ListRuleset() (string, error) {
	r.lists++
	return r.current, r.listErr
}
