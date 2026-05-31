// Bug Condition Exploration Test - Ban Engine Error Propagation & Blocklist Rebuild
// **Validates: Requirements 1.6, 1.7, 1.8**
//
// These tests encode the EXPECTED (correct) behavior. They are expected to FAIL on
// unfixed code, confirming the bugs exist. After the fix is applied, they should PASS.
//
// Bug 6a: SyncToXDP() error is silently discarded with `_ = e.SyncToXDP()`
// Bug 6b: CleanupExpired() removes from memory but not from blocklist file
// Bug 6c: Unban() removes from memory but not from blocklist file
package ban

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// TestBugCondition_SyncToXDP_ErrorNotDiscarded tests that when SyncToXDP() fails
// during Ban(), the error is logged/returned rather than silently discarded.
//
// Bug Condition: `_ = e.SyncToXDP()` discards the error.
// Expected Behavior: error is logged with IP and reason context.
//
// **Validates: Requirements 1.6**
func TestBugCondition_SyncToXDP_ErrorNotDiscarded(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		ip := rapid.StringMatching(`^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$`).Draw(t, "ip")
		reason := rapid.StringMatching(`^[a-z_]{3,20}$`).Draw(t, "reason")

		// Create a temp dir for blocklist
		dir, err := os.MkdirTemp("", "bugfix_test_6a_*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(dir)

		blocklistPath := filepath.Join(dir, "blocklist.txt")

		// Use a sync command that will FAIL (nonexistent command)
		engine := NewInMemoryBanEngine(blocklistPath, "/nonexistent/sync/command/that/will/fail")
		now := time.Now()
		engine.nowFunc = func() time.Time { return now }

		// Capture stderr to check if error is logged
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		// Call Ban - the SyncToXDP will fail
		engine.Ban(ip, 15*time.Minute, reason)

		// Restore stderr and read captured output
		w.Close()
		os.Stderr = oldStderr
		var buf [4096]byte
		n, _ := r.Read(buf[:])
		r.Close()
		stderrOutput := string(buf[:n])

		// Property: When SyncToXDP() fails during Ban(), the error MUST be logged.
		// The bug is that Ban() uses `_ = e.SyncToXDP()` which silently discards the error.
		// After the fix, the error should appear in log output.
		// We check that stderr contains some indication of the sync failure.
		if !strings.Contains(stderrOutput, "SyncToXDP") &&
			!strings.Contains(stderrOutput, "sync") &&
			!strings.Contains(stderrOutput, ip) {
			t.Fatalf("BUG CONFIRMED: SyncToXDP() error silently discarded during Ban(ip=%s, reason=%s). No error logged to stderr. The error from the failing sync command is invisible to operators.", ip, reason)
		}
	})
}

// TestBugCondition_CleanupExpired_RemovesFromBlocklist tests that after CleanupExpired()
// removes expired IPs from memory, the blocklist file also does NOT contain those IPs.
//
// Bug Condition: CleanupExpired removes from memory but stale IPs remain in blocklist file.
// Expected Behavior: blocklist file only contains currently-banned IPs after cleanup.
//
// **Validates: Requirements 1.7**
func TestBugCondition_CleanupExpired_RemovesFromBlocklist(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate IPs to ban
		expiredIP := rapid.StringMatching(`^10\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$`).Draw(t, "expiredIP")
		activeIP := rapid.StringMatching(`^192\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$`).Draw(t, "activeIP")

		// Ensure IPs are different
		if expiredIP == activeIP {
			return // skip this case
		}

		dir, err := os.MkdirTemp("", "bugfix_test_6b_*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(dir)

		blocklistPath := filepath.Join(dir, "blocklist.txt")

		engine := NewInMemoryBanEngine(blocklistPath, "")
		now := time.Now()
		engine.nowFunc = func() time.Time { return now }

		// Ban both IPs - one will expire, one will remain active
		engine.Ban(expiredIP, 1*time.Minute, "will_expire")
		engine.Ban(activeIP, 1*time.Hour, "stays_active")

		// Advance time past the expiration of the first IP
		now = now.Add(2 * time.Minute)
		engine.nowFunc = func() time.Time { return now }

		// Call CleanupExpired - should remove expiredIP from memory AND blocklist
		engine.CleanupExpired()

		// Verify expiredIP is removed from memory
		if engine.IsBanned(expiredIP) {
			t.Fatalf("expiredIP %s should not be banned after cleanup", expiredIP)
		}

		// Verify activeIP is still banned
		if !engine.IsBanned(activeIP) {
			t.Fatalf("activeIP %s should still be banned after cleanup", activeIP)
		}

		// Property: The blocklist file should NOT contain the expired IP
		content, err := os.ReadFile(blocklistPath)
		if err != nil {
			t.Fatalf("failed to read blocklist file: %v", err)
		}

		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if trimmed == expiredIP+"/32" {
				t.Fatalf("BUG CONFIRMED: Blocklist file still contains expired IP %s/32 after CleanupExpired(). Stale entries remain in blocklist file, causing unbounded growth and incorrect XDP blocking.", expiredIP)
			}
		}
	})
}

// TestBugCondition_Unban_RemovesFromBlocklist tests that after Unban() removes an IP
// from memory, the blocklist file also does NOT contain that IP.
//
// Bug Condition: Unban removes from memory but not from blocklist file.
// Expected Behavior: blocklist file does not contain unbanned IP, XDP sync triggered.
//
// **Validates: Requirements 1.8**
func TestBugCondition_Unban_RemovesFromBlocklist(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate IPs
		unbannedIP := rapid.StringMatching(`^10\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$`).Draw(t, "unbannedIP")
		remainingIP := rapid.StringMatching(`^192\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$`).Draw(t, "remainingIP")

		// Ensure IPs are different
		if unbannedIP == remainingIP {
			return // skip this case
		}

		dir, err := os.MkdirTemp("", "bugfix_test_6c_*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(dir)

		blocklistPath := filepath.Join(dir, "blocklist.txt")

		engine := NewInMemoryBanEngine(blocklistPath, "")
		now := time.Now()
		engine.nowFunc = func() time.Time { return now }

		// Ban both IPs
		engine.Ban(unbannedIP, 1*time.Hour, "to_be_unbanned")
		engine.Ban(remainingIP, 1*time.Hour, "stays_banned")

		// Unban one IP
		engine.Unban(unbannedIP)

		// Verify unbannedIP is removed from memory
		if engine.IsBanned(unbannedIP) {
			t.Fatalf("unbannedIP %s should not be banned after Unban()", unbannedIP)
		}

		// Verify remainingIP is still banned
		if !engine.IsBanned(remainingIP) {
			t.Fatalf("remainingIP %s should still be banned", remainingIP)
		}

		// Property: The blocklist file should NOT contain the unbanned IP
		content, err := os.ReadFile(blocklistPath)
		if err != nil {
			t.Fatalf("failed to read blocklist file: %v", err)
		}

		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if trimmed == unbannedIP+"/32" {
				t.Fatalf("BUG CONFIRMED: Blocklist file still contains unbanned IP %s/32 after Unban(). Stale entry remains in blocklist file, causing incorrect XDP blocking of previously-banned IP.", unbannedIP)
			}
		}
	})
}
