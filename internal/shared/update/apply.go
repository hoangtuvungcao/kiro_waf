package update

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kiro_waf/internal/shared/storage"
)

const pendingUpdateFile = "pending-update.json"

type ApplyOptions struct {
	TargetPath   string
	ArtifactPath string
	StateDir     string
	Version      string
	HealthCheck  func() error
	Now          time.Time
}

type PendingUpdate struct {
	TargetPath   string `json:"target_path"`
	ArtifactPath string `json:"artifact_path"`
	SnapshotPath string `json:"snapshot_path"`
	Version      string `json:"version"`
	AppliedAt    string `json:"applied_at"`
}

type ApplyResult struct {
	PendingPath  string `json:"pending_path"`
	SnapshotPath string `json:"snapshot_path"`
	RolledBack   bool   `json:"rolled_back"`
}

func ApplyFileWithRollback(opts ApplyOptions) (ApplyResult, error) {
	if strings.TrimSpace(opts.TargetPath) == "" {
		return ApplyResult{}, errors.New("update target path is required")
	}
	if strings.TrimSpace(opts.ArtifactPath) == "" {
		return ApplyResult{}, errors.New("update artifact path is required")
	}
	if strings.TrimSpace(opts.StateDir) == "" {
		return ApplyResult{}, errors.New("update state dir is required")
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}

	rollbackDir := filepath.Join(opts.StateDir, "update-rollback")
	if err := os.MkdirAll(rollbackDir, 0o755); err != nil {
		return ApplyResult{}, err
	}
	snapshotPath := filepath.Join(rollbackDir, filepath.Base(opts.TargetPath)+"."+opts.Now.Format("20060102T150405Z")+".bak")
	if err := copyFile(snapshotPath, opts.TargetPath, 0o644); err != nil {
		return ApplyResult{}, err
	}
	pending := PendingUpdate{
		TargetPath:   opts.TargetPath,
		ArtifactPath: opts.ArtifactPath,
		SnapshotPath: snapshotPath,
		Version:      opts.Version,
		AppliedAt:    opts.Now.Format(time.RFC3339),
	}
	pendingPath := filepath.Join(opts.StateDir, pendingUpdateFile)
	if err := storage.WriteJSONAtomic(pendingPath, pending); err != nil {
		return ApplyResult{}, err
	}
	if err := copyFile(opts.TargetPath, opts.ArtifactPath, 0o755); err != nil {
		_ = RollbackPending(opts.StateDir)
		return ApplyResult{PendingPath: pendingPath, SnapshotPath: snapshotPath, RolledBack: true}, err
	}
	if opts.HealthCheck != nil {
		if err := opts.HealthCheck(); err != nil {
			if rollbackErr := RollbackPending(opts.StateDir); rollbackErr != nil {
				return ApplyResult{PendingPath: pendingPath, SnapshotPath: snapshotPath, RolledBack: true}, rollbackErr
			}
			return ApplyResult{PendingPath: pendingPath, SnapshotPath: snapshotPath, RolledBack: true}, err
		}
	}
	return ApplyResult{PendingPath: pendingPath, SnapshotPath: snapshotPath}, nil
}

func ConfirmPending(stateDir string) error {
	if strings.TrimSpace(stateDir) == "" {
		return errors.New("update state dir is required")
	}
	err := os.Remove(filepath.Join(stateDir, pendingUpdateFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func RollbackPending(stateDir string) error {
	if strings.TrimSpace(stateDir) == "" {
		return errors.New("update state dir is required")
	}
	pending, err := ReadPending(stateDir)
	if err != nil {
		return err
	}
	if strings.TrimSpace(pending.TargetPath) == "" || strings.TrimSpace(pending.SnapshotPath) == "" {
		return errors.New("pending update is missing rollback paths")
	}
	if err := copyFile(pending.TargetPath, pending.SnapshotPath, 0o755); err != nil {
		return err
	}
	return os.Remove(filepath.Join(stateDir, pendingUpdateFile))
}

func ReadPending(stateDir string) (PendingUpdate, error) {
	var pending PendingUpdate
	err := storage.ReadJSON(filepath.Join(stateDir, pendingUpdateFile), &pending)
	return pending, err
}

func copyFile(dst string, src string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".tmp-update-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}
