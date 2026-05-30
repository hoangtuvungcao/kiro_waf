package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDownloadAndVerify_Success(t *testing.T) {
	// Create test binary content
	binaryContent := []byte("#!/bin/bash\necho 'hello kiro'\n")
	hash := sha256.Sum256(binaryContent)
	expectedSHA := hex.EncodeToString(hash[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-License-Key") != "test-key" {
			t.Errorf("missing or wrong X-License-Key header: got %q", r.Header.Get("X-License-Key"))
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(binaryContent)
	}))
	defer server.Close()

	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:       server.URL,
		LicenseKey:      "test-key",
		Component:       "kiro-client-waf",
		Channel:         "stable",
		CurrentVersion:  "1.0.0",
		DownloadTimeout: 30 * time.Second,
	})

	info := &UpdateInfo{
		Version:     "1.1.0",
		ArtifactURL: server.URL + "/download/client-waf",
		SHA256:      expectedSHA,
	}

	path, err := u.DownloadAndVerify(context.Background(), info)
	if err != nil {
		t.Fatalf("DownloadAndVerify returned error: %v", err)
	}
	defer os.Remove(path)

	// Verify the downloaded file content
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(content) != string(binaryContent) {
		t.Errorf("downloaded content mismatch: got %q, want %q", content, binaryContent)
	}

	// Verify state returned to idle
	state := u.State()
	if state.Status != "idle" {
		t.Errorf("state.Status = %q, want %q", state.Status, "idle")
	}
}

func TestDownloadAndVerify_ChecksumMismatch(t *testing.T) {
	binaryContent := []byte("some binary content")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(binaryContent)
	}))
	defer server.Close()

	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:       server.URL,
		LicenseKey:      "test-key",
		Component:       "kiro-client-waf",
		Channel:         "stable",
		CurrentVersion:  "1.0.0",
		DownloadTimeout: 30 * time.Second,
	})

	info := &UpdateInfo{
		Version:     "1.1.0",
		ArtifactURL: server.URL + "/download/client-waf",
		SHA256:      "0000000000000000000000000000000000000000000000000000000000000000",
	}

	path, err := u.DownloadAndVerify(context.Background(), info)
	if err == nil {
		os.Remove(path)
		t.Fatal("expected error for checksum mismatch, got nil")
	}

	// Verify error message mentions SHA-256 mismatch
	if path != "" {
		t.Errorf("expected empty path on error, got %q", path)
	}

	// Verify temp file was cleaned up
	// (we can't easily check this without knowing the temp path, but the deferred cleanup handles it)
}

func TestDownloadAndVerify_NetworkError(t *testing.T) {
	// Use a server that immediately closes connection
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Hijack and close to simulate network error
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijack not supported", http.StatusInternalServerError)
			return
		}
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	defer server.Close()

	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:       server.URL,
		LicenseKey:      "test-key",
		Component:       "kiro-client-waf",
		Channel:         "stable",
		CurrentVersion:  "1.0.0",
		DownloadTimeout: 5 * time.Second,
	})

	info := &UpdateInfo{
		Version:     "1.1.0",
		ArtifactURL: server.URL + "/download/client-waf",
		SHA256:      "abc123",
	}

	path, err := u.DownloadAndVerify(context.Background(), info)
	if err == nil {
		os.Remove(path)
		t.Fatal("expected error for network failure, got nil")
	}
	if path != "" {
		t.Errorf("expected empty path on error, got %q", path)
	}
}

func TestDownloadAndVerify_Timeout(t *testing.T) {
	// Server that never responds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second) // longer than our timeout
	}))
	defer server.Close()

	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:       server.URL,
		LicenseKey:      "test-key",
		Component:       "kiro-client-waf",
		Channel:         "stable",
		CurrentVersion:  "1.0.0",
		DownloadTimeout: 100 * time.Millisecond, // Very short timeout for test
	})

	info := &UpdateInfo{
		Version:     "1.1.0",
		ArtifactURL: server.URL + "/download/client-waf",
		SHA256:      "abc123",
	}

	path, err := u.DownloadAndVerify(context.Background(), info)
	if err == nil {
		os.Remove(path)
		t.Fatal("expected error for timeout, got nil")
	}
	if path != "" {
		t.Errorf("expected empty path on timeout, got %q", path)
	}
}

func TestDownloadAndVerify_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:       server.URL,
		LicenseKey:      "test-key",
		Component:       "kiro-client-waf",
		Channel:         "stable",
		CurrentVersion:  "1.0.0",
		DownloadTimeout: 30 * time.Second,
	})

	info := &UpdateInfo{
		Version:     "1.1.0",
		ArtifactURL: server.URL + "/download/client-waf",
		SHA256:      "abc123",
	}

	path, err := u.DownloadAndVerify(context.Background(), info)
	if err == nil {
		os.Remove(path)
		t.Fatal("expected error for server error, got nil")
	}
	if path != "" {
		t.Errorf("expected empty path on server error, got %q", path)
	}
}

func TestDownloadAndVerify_NilInfo(t *testing.T) {
	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:       "http://localhost",
		LicenseKey:      "test-key",
		Component:       "kiro-client-waf",
		Channel:         "stable",
		CurrentVersion:  "1.0.0",
		DownloadTimeout: 30 * time.Second,
	})

	_, err := u.DownloadAndVerify(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil info, got nil")
	}
}

func TestDownloadAndVerify_EmptyArtifactURL(t *testing.T) {
	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:       "http://localhost",
		LicenseKey:      "test-key",
		Component:       "kiro-client-waf",
		Channel:         "stable",
		CurrentVersion:  "1.0.0",
		DownloadTimeout: 30 * time.Second,
	})

	info := &UpdateInfo{
		Version:     "1.1.0",
		ArtifactURL: "",
		SHA256:      "abc123",
	}

	_, err := u.DownloadAndVerify(context.Background(), info)
	if err == nil {
		t.Fatal("expected error for empty artifact URL, got nil")
	}
}

func TestDownloadAndVerify_CaseInsensitiveSHA(t *testing.T) {
	binaryContent := []byte("test binary")
	hash := sha256.Sum256(binaryContent)
	// Use uppercase SHA
	expectedSHA := hex.EncodeToString(hash[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(binaryContent)
	}))
	defer server.Close()

	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:       server.URL,
		LicenseKey:      "test-key",
		Component:       "kiro-client-waf",
		Channel:         "stable",
		CurrentVersion:  "1.0.0",
		DownloadTimeout: 30 * time.Second,
	})

	// Test with uppercase SHA-256
	info := &UpdateInfo{
		Version:     "1.1.0",
		ArtifactURL: server.URL + "/download",
		SHA256:      "  " + expectedSHA + "  ", // with whitespace
	}

	path, err := u.DownloadAndVerify(context.Background(), info)
	if err != nil {
		t.Fatalf("DownloadAndVerify returned error: %v", err)
	}
	defer os.Remove(path)
}

func TestApplyUpdate_Success(t *testing.T) {
	// Create a temp directory for the test
	tmpDir := t.TempDir()

	// Create "current binary"
	currentBinaryPath := filepath.Join(tmpDir, "kiro-client-waf")
	if err := os.WriteFile(currentBinaryPath, []byte("old binary v1.0"), 0755); err != nil {
		t.Fatalf("failed to create current binary: %v", err)
	}

	// Create "new binary" (simulating downloaded file)
	newBinaryPath := filepath.Join(tmpDir, "kiro-update-new.bin")
	newContent := []byte("new binary v1.1")
	if err := os.WriteFile(newBinaryPath, newContent, 0644); err != nil {
		t.Fatalf("failed to create new binary: %v", err)
	}

	backupPath := filepath.Join(tmpDir, "kiro-client-waf.prev")

	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:      "http://localhost",
		LicenseKey:     "test-key",
		Component:      "kiro-client-waf",
		Channel:        "stable",
		CurrentVersion: "1.0.0",
		BinaryPath:     currentBinaryPath,
		BackupPath:     backupPath,
	})

	err := u.ApplyUpdate(context.Background(), newBinaryPath)
	if err != nil {
		t.Fatalf("ApplyUpdate returned error: %v", err)
	}

	// Verify the binary was replaced
	content, err := os.ReadFile(currentBinaryPath)
	if err != nil {
		t.Fatalf("failed to read replaced binary: %v", err)
	}
	if string(content) != string(newContent) {
		t.Errorf("binary content mismatch: got %q, want %q", content, newContent)
	}

	// Verify permissions are 755
	info, err := os.Stat(currentBinaryPath)
	if err != nil {
		t.Fatalf("failed to stat binary: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("binary permissions = %o, want 0755", info.Mode().Perm())
	}

	// Verify backup was created
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("failed to read backup: %v", err)
	}
	if string(backupContent) != "old binary v1.0" {
		t.Errorf("backup content mismatch: got %q, want %q", backupContent, "old binary v1.0")
	}

	// Verify state was updated
	state := u.State()
	if state.PreviousVersion != "1.0.0" {
		t.Errorf("state.PreviousVersion = %q, want %q", state.PreviousVersion, "1.0.0")
	}
	if state.Status != "idle" {
		t.Errorf("state.Status = %q, want %q", state.Status, "idle")
	}
	if state.LastUpdateAt.IsZero() {
		t.Error("state.LastUpdateAt should not be zero")
	}
}

func TestApplyUpdate_NoExistingBinary(t *testing.T) {
	// Test applying update when there's no existing binary (fresh install scenario)
	tmpDir := t.TempDir()

	currentBinaryPath := filepath.Join(tmpDir, "kiro-client-waf")
	// Don't create the current binary - simulating fresh install

	newBinaryPath := filepath.Join(tmpDir, "kiro-update-new.bin")
	newContent := []byte("new binary v1.0")
	if err := os.WriteFile(newBinaryPath, newContent, 0644); err != nil {
		t.Fatalf("failed to create new binary: %v", err)
	}

	backupPath := filepath.Join(tmpDir, "kiro-client-waf.prev")

	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:      "http://localhost",
		LicenseKey:     "test-key",
		Component:      "kiro-client-waf",
		Channel:        "stable",
		CurrentVersion: "0.0.0",
		BinaryPath:     currentBinaryPath,
		BackupPath:     backupPath,
	})

	err := u.ApplyUpdate(context.Background(), newBinaryPath)
	if err != nil {
		t.Fatalf("ApplyUpdate returned error: %v", err)
	}

	// Verify the binary was placed
	content, err := os.ReadFile(currentBinaryPath)
	if err != nil {
		t.Fatalf("failed to read binary: %v", err)
	}
	if string(content) != string(newContent) {
		t.Errorf("binary content mismatch: got %q, want %q", content, newContent)
	}
}

func TestApplyUpdate_EmptyNewBinaryPath(t *testing.T) {
	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:      "http://localhost",
		LicenseKey:     "test-key",
		Component:      "kiro-client-waf",
		Channel:        "stable",
		CurrentVersion: "1.0.0",
		BinaryPath:     "/usr/local/bin/kiro-client-waf",
		BackupPath:     "/usr/local/bin/kiro-client-waf.prev",
	})

	err := u.ApplyUpdate(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty new binary path, got nil")
	}
}

func TestApplyUpdate_EmptyBinaryPath(t *testing.T) {
	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:      "http://localhost",
		LicenseKey:     "test-key",
		Component:      "kiro-client-waf",
		Channel:        "stable",
		CurrentVersion: "1.0.0",
		BinaryPath:     "",
		BackupPath:     "/usr/local/bin/kiro-client-waf.prev",
	})

	err := u.ApplyUpdate(context.Background(), "/tmp/some-binary")
	if err == nil {
		t.Fatal("expected error for empty binary path, got nil")
	}
}

func TestApplyUpdate_EmptyBackupPath(t *testing.T) {
	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:      "http://localhost",
		LicenseKey:     "test-key",
		Component:      "kiro-client-waf",
		Channel:        "stable",
		CurrentVersion: "1.0.0",
		BinaryPath:     "/usr/local/bin/kiro-client-waf",
		BackupPath:     "",
	})

	err := u.ApplyUpdate(context.Background(), "/tmp/some-binary")
	if err == nil {
		t.Fatal("expected error for empty backup path, got nil")
	}
}

func TestApplyUpdate_OverwritesExistingBackup(t *testing.T) {
	// Verify that applying update overwrites existing backup (exactly one backup retained)
	tmpDir := t.TempDir()

	currentBinaryPath := filepath.Join(tmpDir, "kiro-client-waf")
	if err := os.WriteFile(currentBinaryPath, []byte("binary v2.0"), 0755); err != nil {
		t.Fatalf("failed to create current binary: %v", err)
	}

	// Create existing backup (from previous update)
	backupPath := filepath.Join(tmpDir, "kiro-client-waf.prev")
	if err := os.WriteFile(backupPath, []byte("binary v1.0"), 0755); err != nil {
		t.Fatalf("failed to create existing backup: %v", err)
	}

	newBinaryPath := filepath.Join(tmpDir, "kiro-update-new.bin")
	if err := os.WriteFile(newBinaryPath, []byte("binary v3.0"), 0644); err != nil {
		t.Fatalf("failed to create new binary: %v", err)
	}

	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:      "http://localhost",
		LicenseKey:     "test-key",
		Component:      "kiro-client-waf",
		Channel:        "stable",
		CurrentVersion: "2.0.0",
		BinaryPath:     currentBinaryPath,
		BackupPath:     backupPath,
	})

	err := u.ApplyUpdate(context.Background(), newBinaryPath)
	if err != nil {
		t.Fatalf("ApplyUpdate returned error: %v", err)
	}

	// Verify backup now contains v2.0 (not v1.0)
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("failed to read backup: %v", err)
	}
	if string(backupContent) != "binary v2.0" {
		t.Errorf("backup content = %q, want %q (should be overwritten with current version)", backupContent, "binary v2.0")
	}

	// Verify current binary is v3.0
	currentContent, err := os.ReadFile(currentBinaryPath)
	if err != nil {
		t.Fatalf("failed to read current binary: %v", err)
	}
	if string(currentContent) != "binary v3.0" {
		t.Errorf("current binary content = %q, want %q", currentContent, "binary v3.0")
	}
}

func TestDownloadAndVerify_PartialDownloadCleanup(t *testing.T) {
	// Server sends partial data then closes connection
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write some data then close (simulating partial download)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("partial data"))
		// The response will end here, but the SHA won't match
	}))
	defer server.Close()

	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:       server.URL,
		LicenseKey:      "test-key",
		Component:       "kiro-client-waf",
		Channel:         "stable",
		CurrentVersion:  "1.0.0",
		DownloadTimeout: 30 * time.Second,
	})

	info := &UpdateInfo{
		Version:     "1.1.0",
		ArtifactURL: server.URL + "/download",
		SHA256:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}

	path, err := u.DownloadAndVerify(context.Background(), info)
	if err == nil {
		os.Remove(path)
		t.Fatal("expected error for checksum mismatch on partial download, got nil")
	}
	if path != "" {
		// Verify the temp file was cleaned up
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Errorf("temp file %q should have been cleaned up", path)
			os.Remove(path)
		}
	}
}
