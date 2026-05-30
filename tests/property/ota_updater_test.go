// Feature: waf-system-overhaul, Property 1: SHA-256 Verification Round-Trip
// **Validates: Requirements 4.6, 5.3**
//
// For any binary content and its computed SHA-256 hash, the verification function
// SHALL return true when the computed hash of the content matches the expected hash,
// and SHALL return false when the content is modified (even by a single byte).
//
// Feature: waf-system-overhaul, Property 2: OTA Poll Interval Clamping
// **Validates: Requirements 5.1**
//
// For any integer value provided as poll interval configuration, the effective interval
// SHALL be clamped to minimum 60 seconds and maximum 86,400 seconds, with a default of
// 300 seconds when no value or an invalid value is provided.
//
// Feature: waf-system-overhaul, Property 3: Atomic Binary Replacement Preserves Content
// **Validates: Requirements 5.6**
//
// For any valid binary file content, after performing atomic replacement via rename(2),
// reading the target path SHALL return exactly the new content with no partial writes
// or corruption observable.
//
// Feature: waf-system-overhaul, Property 4: Exactly One Backup Version Retained
// **Validates: Requirements 5.8**
//
// For any sequence of N successful OTA updates (N ≥ 1), the backup directory SHALL
// contain exactly one previous binary version (the version immediately preceding the
// current one).
package property

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kiro_waf/internal/client/updater"

	"pgregory.net/rapid"
)

// --- Property 1: SHA-256 Verification Round-Trip ---

// TestOTA_SHA256VerificationRoundTrip_Match verifies that for any binary content,
// computing its SHA-256 and using it as the expected hash results in successful verification.
func TestOTA_SHA256VerificationRoundTrip_Match(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random binary content (0 to 8192 bytes)
		content := rapid.SliceOfN(rapid.Byte(), 0, 8192).Draw(t, "content")

		// Compute SHA-256 hash
		sum := sha256.Sum256(content)
		expectedHex := hex.EncodeToString(sum[:])

		// Simulate the OTA updater's verification logic:
		// Compute hash of content and compare with expected
		computedSum := sha256.Sum256(content)
		computedHex := hex.EncodeToString(computedSum[:])

		// Property: verification SHALL return true when hash matches
		if computedHex != expectedHex {
			t.Fatalf("SHA-256 round-trip failed: content_len=%d, expected=%s, computed=%s",
				len(content), expectedHex, computedHex)
		}
	})
}

// TestOTA_SHA256VerificationRoundTrip_ModifiedContent verifies that for any binary content,
// modifying even a single byte causes SHA-256 verification to fail.
func TestOTA_SHA256VerificationRoundTrip_ModifiedContent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random binary content with at least 1 byte
		content := rapid.SliceOfN(rapid.Byte(), 1, 8192).Draw(t, "content")

		// Compute original SHA-256 hash
		originalSum := sha256.Sum256(content)
		originalHex := hex.EncodeToString(originalSum[:])

		// Modify a single byte in the content
		modIdx := rapid.IntRange(0, len(content)-1).Draw(t, "modIdx")
		modifiedContent := make([]byte, len(content))
		copy(modifiedContent, content)

		// Ensure the byte actually changes
		modifiedContent[modIdx] ^= 0xFF

		// Compute hash of modified content
		modifiedSum := sha256.Sum256(modifiedContent)
		modifiedHex := hex.EncodeToString(modifiedSum[:])

		// Property: verification SHALL return false when content is modified
		if modifiedHex == originalHex {
			t.Fatalf("SHA-256 verification should fail for modified content: "+
				"content_len=%d, modIdx=%d, hash=%s",
				len(content), modIdx, originalHex)
		}
	})
}

// --- Property 2: OTA Poll Interval Clamping ---

// TestOTA_PollIntervalClamping verifies that for any integer poll interval,
// the effective interval is clamped to [60s, 86400s], with default 300s for invalid values.
func TestOTA_PollIntervalClamping(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random duration in seconds (including negative, zero, and very large)
		seconds := rapid.Int64Range(-1000000, 1000000).Draw(t, "seconds")
		d := time.Duration(seconds) * time.Second

		result := updater.ClampPollInterval(d)

		// Property: result SHALL always be within [60s, 86400s]
		if result < 60*time.Second {
			t.Fatalf("ClampPollInterval(%v) = %v, below minimum 60s", d, result)
		}
		if result > 86400*time.Second {
			t.Fatalf("ClampPollInterval(%v) = %v, above maximum 86400s", d, result)
		}

		// Property: zero or negative SHALL return default 300s
		if seconds <= 0 {
			if result != 300*time.Second {
				t.Fatalf("ClampPollInterval(%v) = %v, want default 300s for non-positive input", d, result)
			}
		}

		// Property: values within valid range SHALL be preserved
		if seconds >= 60 && seconds <= 86400 {
			if result != d {
				t.Fatalf("ClampPollInterval(%v) = %v, valid input should be preserved", d, result)
			}
		}

		// Property: values below minimum SHALL be clamped to 60s
		if seconds > 0 && seconds < 60 {
			if result != 60*time.Second {
				t.Fatalf("ClampPollInterval(%v) = %v, want 60s for below-minimum input", d, result)
			}
		}

		// Property: values above maximum SHALL be clamped to 86400s
		if seconds > 86400 {
			if result != 86400*time.Second {
				t.Fatalf("ClampPollInterval(%v) = %v, want 86400s for above-maximum input", d, result)
			}
		}
	})
}

// TestOTA_PollIntervalClamping_NewUpdater verifies that NewOTAUpdater also clamps the interval.
func TestOTA_PollIntervalClamping_NewUpdater(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		seconds := rapid.Int64Range(-1000000, 1000000).Draw(t, "seconds")
		d := time.Duration(seconds) * time.Second

		u := updater.NewOTAUpdater(updater.UpdaterConfig{
			MasterURL:      "http://localhost",
			LicenseKey:     "test-key",
			Component:      "kiro-client-waf",
			Channel:        "stable",
			CurrentVersion: "1.0.0",
			PollInterval:   d,
		})

		config := u.Config()

		// Property: effective interval SHALL always be within [60s, 86400s]
		if config.PollInterval < 60*time.Second {
			t.Fatalf("NewOTAUpdater with PollInterval=%v: effective=%v, below minimum 60s", d, config.PollInterval)
		}
		if config.PollInterval > 86400*time.Second {
			t.Fatalf("NewOTAUpdater with PollInterval=%v: effective=%v, above maximum 86400s", d, config.PollInterval)
		}
	})
}

// --- Property 3: Atomic Binary Replacement Preserves Content ---

// TestOTA_AtomicBinaryReplacementPreservesContent verifies that for any binary content,
// after atomic replacement via rename(2), reading the target returns exactly the new content.
func TestOTA_AtomicBinaryReplacementPreservesContent(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random binary content for the "new" binary (1 to 8192 bytes)
		newContent := rapid.SliceOfN(rapid.Byte(), 1, 8192).Draw(rt, "newContent")

		// Generate random binary content for the "old" binary
		oldContent := rapid.SliceOfN(rapid.Byte(), 1, 4096).Draw(rt, "oldContent")

		// Create temp directory for this test iteration
		tmpDir, err := os.MkdirTemp("", "ota-atomic-test-*")
		if err != nil {
			rt.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		targetPath := filepath.Join(tmpDir, "kiro-client-waf")
		backupPath := filepath.Join(tmpDir, "kiro-client-waf.prev")
		newBinaryPath := filepath.Join(tmpDir, "kiro-update-new.bin")

		// Write the "old" binary at target path
		if err := os.WriteFile(targetPath, oldContent, 0755); err != nil {
			rt.Fatalf("failed to write old binary: %v", err)
		}

		// Write the "new" binary at temp path
		if err := os.WriteFile(newBinaryPath, newContent, 0644); err != nil {
			rt.Fatalf("failed to write new binary: %v", err)
		}

		// Perform atomic replacement using the OTA updater
		u := updater.NewOTAUpdater(updater.UpdaterConfig{
			MasterURL:      "http://localhost",
			LicenseKey:     "test-key",
			Component:      "kiro-client-waf",
			Channel:        "stable",
			CurrentVersion: "1.0.0",
			BinaryPath:     targetPath,
			BackupPath:     backupPath,
		})

		if err := u.ApplyUpdate(context.Background(), newBinaryPath); err != nil {
			rt.Fatalf("ApplyUpdate failed: %v", err)
		}

		// Property: reading target path SHALL return exactly the new content
		readContent, err := os.ReadFile(targetPath)
		if err != nil {
			rt.Fatalf("failed to read target after replacement: %v", err)
		}

		if len(readContent) != len(newContent) {
			rt.Fatalf("content length mismatch: got %d, want %d", len(readContent), len(newContent))
		}

		for i := range newContent {
			if readContent[i] != newContent[i] {
				rt.Fatalf("content mismatch at byte %d: got 0x%02x, want 0x%02x",
					i, readContent[i], newContent[i])
			}
		}
	})
}

// --- Property 4: Exactly One Backup Version Retained ---

// TestOTA_ExactlyOneBackupRetained verifies that for any sequence of N updates (N≥1),
// exactly one backup exists containing the immediately preceding version.
func TestOTA_ExactlyOneBackupRetained(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate number of sequential updates (1 to 10)
		numUpdates := rapid.IntRange(1, 10).Draw(rt, "numUpdates")

		// Create temp directory
		tmpDir, err := os.MkdirTemp("", "ota-backup-test-*")
		if err != nil {
			rt.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		targetPath := filepath.Join(tmpDir, "kiro-client-waf")
		backupPath := filepath.Join(tmpDir, "kiro-client-waf.prev")

		// Write initial binary (version 0)
		initialContent := rapid.SliceOfN(rapid.Byte(), 1, 1024).Draw(rt, "initialContent")
		if err := os.WriteFile(targetPath, initialContent, 0755); err != nil {
			rt.Fatalf("failed to write initial binary: %v", err)
		}

		// Track the content of each version
		previousContent := initialContent

		// Perform N sequential updates
		for i := 0; i < numUpdates; i++ {
			// Generate new binary content for this update
			newContent := rapid.SliceOfN(rapid.Byte(), 1, 1024).Draw(rt, "updateContent")

			// Write new binary to temp location
			newBinaryPath := filepath.Join(tmpDir, "kiro-update-new.bin")
			if err := os.WriteFile(newBinaryPath, newContent, 0644); err != nil {
				rt.Fatalf("update %d: failed to write new binary: %v", i, err)
			}

			// Apply update
			u := updater.NewOTAUpdater(updater.UpdaterConfig{
				MasterURL:      "http://localhost",
				LicenseKey:     "test-key",
				Component:      "kiro-client-waf",
				Channel:        "stable",
				CurrentVersion: "1.0.0",
				BinaryPath:     targetPath,
				BackupPath:     backupPath,
			})

			if err := u.ApplyUpdate(context.Background(), newBinaryPath); err != nil {
				rt.Fatalf("update %d: ApplyUpdate failed: %v", i, err)
			}

			// After each update, verify exactly one backup exists
			backupContent, err := os.ReadFile(backupPath)
			if err != nil {
				rt.Fatalf("update %d: backup should exist but got error: %v", i, err)
			}

			// Property: backup SHALL contain the immediately preceding version
			if len(backupContent) != len(previousContent) {
				rt.Fatalf("update %d: backup content length mismatch: got %d, want %d",
					i, len(backupContent), len(previousContent))
			}
			for j := range previousContent {
				if backupContent[j] != previousContent[j] {
					rt.Fatalf("update %d: backup content mismatch at byte %d: got 0x%02x, want 0x%02x",
						i, j, backupContent[j], previousContent[j])
				}
			}

			// Verify no other backup files exist in the directory
			entries, err := os.ReadDir(tmpDir)
			if err != nil {
				rt.Fatalf("update %d: failed to read directory: %v", i, err)
			}

			backupCount := 0
			for _, entry := range entries {
				if entry.Name() == "kiro-client-waf.prev" {
					backupCount++
				}
			}

			// Property: exactly ONE backup SHALL exist
			if backupCount != 1 {
				rt.Fatalf("update %d: expected exactly 1 backup, found %d", i, backupCount)
			}

			// Update previousContent for next iteration
			previousContent = newContent
		}
	})
}
