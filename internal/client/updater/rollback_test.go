package updater

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOTAUpdater_Rollback_Success(t *testing.T) {
	// Setup temp directory with binary and backup
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "kiro-client-waf")
	backupPath := filepath.Join(tmpDir, "kiro-client-waf.prev")

	// Create current binary
	if err := os.WriteFile(binaryPath, []byte("new-binary-v2"), 0755); err != nil {
		t.Fatal(err)
	}
	// Create backup (previous version)
	if err := os.WriteFile(backupPath, []byte("old-binary-v1"), 0755); err != nil {
		t.Fatal(err)
	}

	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:      "http://localhost",
		LicenseKey:     "test-key",
		Component:      "kiro-client-waf",
		Channel:        "stable",
		CurrentVersion: "2.0.0",
		BinaryPath:     binaryPath,
		BackupPath:     backupPath,
	})

	// Set previous version in state
	u.mu.Lock()
	u.state.PreviousVersion = "1.0.0"
	u.mu.Unlock()

	err := u.Rollback(context.Background())
	if err != nil {
		t.Fatalf("Rollback returned error: %v", err)
	}

	// Verify binary was restored from backup
	content, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("failed to read binary after rollback: %v", err)
	}
	if string(content) != "old-binary-v1" {
		t.Errorf("binary content = %q, want %q", string(content), "old-binary-v1")
	}

	// Verify backup no longer exists (it was renamed)
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Error("backup file should not exist after rollback (it was renamed to binary path)")
	}

	// Verify state was updated
	state := u.State()
	if state.Status != "idle" {
		t.Errorf("Status = %q, want %q", state.Status, "idle")
	}
	if state.CurrentVersion != "1.0.0" {
		t.Errorf("CurrentVersion = %q, want %q", state.CurrentVersion, "1.0.0")
	}
	if state.PreviousVersion != "" {
		t.Errorf("PreviousVersion = %q, want empty", state.PreviousVersion)
	}
}

func TestOTAUpdater_Rollback_NoBackup(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "kiro-client-waf")
	backupPath := filepath.Join(tmpDir, "kiro-client-waf.prev")

	// Create current binary but no backup
	if err := os.WriteFile(binaryPath, []byte("current-binary"), 0755); err != nil {
		t.Fatal(err)
	}

	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:      "http://localhost",
		LicenseKey:     "test-key",
		Component:      "kiro-client-waf",
		Channel:        "stable",
		CurrentVersion: "1.0.0",
		BinaryPath:     binaryPath,
		BackupPath:     backupPath,
	})

	err := u.Rollback(context.Background())
	if err == nil {
		t.Fatal("expected error when backup does not exist, got nil")
	}

	// Verify current binary is unchanged
	content, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("failed to read binary: %v", err)
	}
	if string(content) != "current-binary" {
		t.Errorf("binary should be unchanged, got %q", string(content))
	}

	// Verify state is idle
	state := u.State()
	if state.Status != "idle" {
		t.Errorf("Status = %q, want %q after failed rollback", state.Status, "idle")
	}
}

func TestOTAUpdater_Rollback_EmptyBackupPath(t *testing.T) {
	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:      "http://localhost",
		LicenseKey:     "test-key",
		Component:      "kiro-client-waf",
		Channel:        "stable",
		CurrentVersion: "1.0.0",
		BinaryPath:     "/some/path",
		BackupPath:     "",
	})

	err := u.Rollback(context.Background())
	if err == nil {
		t.Fatal("expected error for empty backup path, got nil")
	}
}

func TestOTAUpdater_Rollback_EmptyBinaryPath(t *testing.T) {
	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:      "http://localhost",
		LicenseKey:     "test-key",
		Component:      "kiro-client-waf",
		Channel:        "stable",
		CurrentVersion: "1.0.0",
		BinaryPath:     "",
		BackupPath:     "/some/backup",
	})

	err := u.Rollback(context.Background())
	if err == nil {
		t.Fatal("expected error for empty binary path, got nil")
	}
}

// mockServiceChecker is a test double for ServiceChecker.
type mockServiceChecker struct {
	activeResults  []bool
	activeErrors   []error
	restartErr     error
	activeCalls    int
	restartCalls   int
}

func (m *mockServiceChecker) IsActive(ctx context.Context) (bool, error) {
	idx := m.activeCalls
	m.activeCalls++

	if idx < len(m.activeErrors) && m.activeErrors[idx] != nil {
		return false, m.activeErrors[idx]
	}
	if idx < len(m.activeResults) {
		return m.activeResults[idx], nil
	}
	// Default: not active
	return false, nil
}

func (m *mockServiceChecker) Restart(ctx context.Context) error {
	m.restartCalls++
	return m.restartErr
}

func TestOTAUpdater_HealthCheckAndRollback_ServiceBecomesActive(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "kiro-client-waf")
	backupPath := filepath.Join(tmpDir, "kiro-client-waf.prev")

	if err := os.WriteFile(binaryPath, []byte("new-binary"), 0755); err != nil {
		t.Fatal(err)
	}

	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:      "http://localhost",
		LicenseKey:     "test-key",
		Component:      "kiro-client-waf",
		Channel:        "stable",
		CurrentVersion: "2.0.0",
		BinaryPath:     binaryPath,
		BackupPath:     backupPath,
		HealthTimeout:  10 * time.Second,
	})

	// Service becomes active on second check
	checker := &mockServiceChecker{
		activeResults: []bool{false, true},
	}

	err := u.HealthCheckAndRollback(context.Background(), checker)
	if err != nil {
		t.Fatalf("HealthCheckAndRollback returned error: %v", err)
	}

	if checker.activeCalls < 2 {
		t.Errorf("expected at least 2 IsActive calls, got %d", checker.activeCalls)
	}
	if checker.restartCalls != 0 {
		t.Errorf("expected 0 Restart calls, got %d", checker.restartCalls)
	}
}

func TestOTAUpdater_HealthCheckAndRollback_TimeoutTriggersRollback(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "kiro-client-waf")
	backupPath := filepath.Join(tmpDir, "kiro-client-waf.prev")

	if err := os.WriteFile(binaryPath, []byte("new-binary-v2"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backupPath, []byte("old-binary-v1"), 0755); err != nil {
		t.Fatal(err)
	}

	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:      "http://localhost",
		LicenseKey:     "test-key",
		Component:      "kiro-client-waf",
		Channel:        "stable",
		CurrentVersion: "2.0.0",
		BinaryPath:     binaryPath,
		BackupPath:     backupPath,
		HealthTimeout:  3 * time.Second, // Short timeout for test
	})

	u.mu.Lock()
	u.state.PreviousVersion = "1.0.0"
	u.mu.Unlock()

	// Service never becomes active
	checker := &mockServiceChecker{
		activeResults: []bool{false, false, false, false, false, false, false, false, false, false},
	}

	err := u.HealthCheckAndRollback(context.Background(), checker)
	if err == nil {
		t.Fatal("expected error indicating rollback occurred, got nil")
	}

	// Verify binary was rolled back
	content, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("failed to read binary after rollback: %v", err)
	}
	if string(content) != "old-binary-v1" {
		t.Errorf("binary content = %q, want %q (should be rolled back)", string(content), "old-binary-v1")
	}

	// Verify restart was called
	if checker.restartCalls != 1 {
		t.Errorf("expected 1 Restart call after rollback, got %d", checker.restartCalls)
	}

	// Verify state
	state := u.State()
	if state.CurrentVersion != "1.0.0" {
		t.Errorf("CurrentVersion = %q, want %q after rollback", state.CurrentVersion, "1.0.0")
	}
}

func TestOTAUpdater_HealthCheckAndRollback_ContextCancelled(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "kiro-client-waf")
	backupPath := filepath.Join(tmpDir, "kiro-client-waf.prev")

	if err := os.WriteFile(binaryPath, []byte("new-binary"), 0755); err != nil {
		t.Fatal(err)
	}

	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:      "http://localhost",
		LicenseKey:     "test-key",
		Component:      "kiro-client-waf",
		Channel:        "stable",
		CurrentVersion: "2.0.0",
		BinaryPath:     binaryPath,
		BackupPath:     backupPath,
		HealthTimeout:  30 * time.Second,
	})

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	checker := &mockServiceChecker{
		activeResults: []bool{false},
	}

	err := u.HealthCheckAndRollback(ctx, checker)
	if err == nil {
		t.Fatal("expected error when context is cancelled, got nil")
	}
}

func TestOTAUpdater_HealthCheckAndRollback_ImmediatelyActive(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "kiro-client-waf")
	backupPath := filepath.Join(tmpDir, "kiro-client-waf.prev")

	if err := os.WriteFile(binaryPath, []byte("new-binary"), 0755); err != nil {
		t.Fatal(err)
	}

	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:      "http://localhost",
		LicenseKey:     "test-key",
		Component:      "kiro-client-waf",
		Channel:        "stable",
		CurrentVersion: "2.0.0",
		BinaryPath:     binaryPath,
		BackupPath:     backupPath,
		HealthTimeout:  10 * time.Second,
	})

	// Service is immediately active
	checker := &mockServiceChecker{
		activeResults: []bool{true},
	}

	err := u.HealthCheckAndRollback(context.Background(), checker)
	if err != nil {
		t.Fatalf("HealthCheckAndRollback returned error: %v", err)
	}

	if checker.activeCalls != 1 {
		t.Errorf("expected 1 IsActive call, got %d", checker.activeCalls)
	}
	if checker.restartCalls != 0 {
		t.Errorf("expected 0 Restart calls, got %d", checker.restartCalls)
	}
}
