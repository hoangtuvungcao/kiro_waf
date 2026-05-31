// Feature: waf-challenge-xdp-fix, Property 2: Preservation - Ban Engine Behaviors Unchanged
// **Validates: Requirements 3.3, 3.4, 3.7**
//
// For all active bans, IsBanned() returns true; for all expired bans, returns false.
// For all appendToBlocklist calls, the file contains the IP in IP/32 CIDR format.
// For all SyncToXDP() calls with valid command that succeeds, returns nil.
package ban

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// genIPv4 generates a valid IPv4 address string.
func genIPv4(t *rapid.T, label string) string {
	a := rapid.IntRange(1, 254).Draw(t, label+"_a")
	b := rapid.IntRange(0, 255).Draw(t, label+"_b")
	c := rapid.IntRange(0, 255).Draw(t, label+"_c")
	d := rapid.IntRange(1, 254).Draw(t, label+"_d")
	return intToStr(a) + "." + intToStr(b) + "." + intToStr(c) + "." + intToStr(d)
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	if neg {
		result = "-" + result
	}
	return result
}

// TestPreservation_IsBanned_ActiveBan verifies that for all active bans,
// IsBanned() returns true. This is existing correct behavior that must be preserved.
func TestPreservation_IsBanned_ActiveBan(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		ip := genIPv4(t, "ip")
		durationMinutes := rapid.IntRange(1, 120).Draw(t, "durationMinutes")

		engine := NewInMemoryBanEngine("", "")
		now := time.Now()
		engine.nowFunc = func() time.Time { return now }

		// Add an active ban entry
		engine.mu.Lock()
		engine.store[ip] = BanEntry{
			IP:        ip,
			ExpiresAt: now.Add(time.Duration(durationMinutes) * time.Minute),
			Reason:    "test_active_ban",
		}
		engine.mu.Unlock()

		// Property: For all active bans, IsBanned() MUST return true
		if !engine.IsBanned(ip) {
			t.Fatalf("expected IsBanned(%q)=true for active ban (expires in %d minutes), got false", ip, durationMinutes)
		}
	})
}

// TestPreservation_IsBanned_ExpiredBan verifies that for all expired bans,
// IsBanned() returns false. This is existing correct behavior that must be preserved.
func TestPreservation_IsBanned_ExpiredBan(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		ip := genIPv4(t, "ip")
		expiredMinutes := rapid.IntRange(1, 120).Draw(t, "expiredMinutes")

		engine := NewInMemoryBanEngine("", "")
		now := time.Now()
		engine.nowFunc = func() time.Time { return now }

		// Add an expired ban entry
		engine.mu.Lock()
		engine.store[ip] = BanEntry{
			IP:        ip,
			ExpiresAt: now.Add(-time.Duration(expiredMinutes) * time.Minute),
			Reason:    "test_expired_ban",
		}
		engine.mu.Unlock()

		// Property: For all expired bans, IsBanned() MUST return false
		if engine.IsBanned(ip) {
			t.Fatalf("expected IsBanned(%q)=false for expired ban (expired %d minutes ago), got true", ip, expiredMinutes)
		}
	})
}

// TestPreservation_AppendToBlocklist_CIDRFormat verifies that for all appendToBlocklist
// calls, the file contains the IP in IP/32 CIDR format.
func TestPreservation_AppendToBlocklist_CIDRFormat(t *testing.T) {
	tmpDir := t.TempDir()

	rapid.Check(t, func(t *rapid.T) {
		ip := genIPv4(t, "ip")

		blocklistPath := filepath.Join(tmpDir, "blocklist_"+ip+".txt")

		engine := NewInMemoryBanEngine(blocklistPath, "")

		// Call appendToBlocklist directly
		engine.appendToBlocklist(ip)

		// Read the file and verify format
		content, err := os.ReadFile(blocklistPath)
		if err != nil {
			t.Fatalf("failed to read blocklist file: %v", err)
		}

		expectedLine := ip + "/32"
		lines := strings.Split(strings.TrimSpace(string(content)), "\n")

		// Property: The file MUST contain the IP in IP/32 CIDR format
		found := false
		for _, line := range lines {
			if strings.TrimSpace(line) == expectedLine {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected blocklist to contain %q, got content: %q", expectedLine, string(content))
		}
	})
}

// TestPreservation_SyncToXDP_ValidCommand verifies that for all SyncToXDP() calls
// with a valid command that succeeds, it returns nil.
func TestPreservation_SyncToXDP_ValidCommand(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Use "true" command which always succeeds on Unix systems
		engine := NewInMemoryBanEngine("", "true")

		// Property: SyncToXDP with a valid command that succeeds MUST return nil
		err := engine.SyncToXDP()
		if err != nil {
			t.Fatalf("expected SyncToXDP() to return nil for valid command 'true', got error: %v", err)
		}
	})
}

// TestPreservation_SyncToXDP_EmptyCommand verifies that SyncToXDP with empty command
// returns nil (no-op behavior).
func TestPreservation_SyncToXDP_EmptyCommand(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		engine := NewInMemoryBanEngine("", "")

		// Property: SyncToXDP with empty command MUST return nil (no-op)
		err := engine.SyncToXDP()
		if err != nil {
			t.Fatalf("expected SyncToXDP() to return nil for empty command, got error: %v", err)
		}
	})
}

// TestPreservation_AppendToBlocklist_MultipleIPs verifies that multiple appendToBlocklist
// calls produce correct IP/32 entries for each IP.
func TestPreservation_AppendToBlocklist_MultipleIPs(t *testing.T) {
	tmpDir := t.TempDir()
	counter := 0

	rapid.Check(t, func(t *rapid.T) {
		numIPs := rapid.IntRange(1, 10).Draw(t, "numIPs")

		counter++
		blocklistPath := filepath.Join(tmpDir, "blocklist_"+intToStr(counter)+".txt")

		engine := NewInMemoryBanEngine(blocklistPath, "")

		ips := make([]string, numIPs)
		for i := 0; i < numIPs; i++ {
			ips[i] = genIPv4(t, "ip_"+intToStr(i))
			engine.appendToBlocklist(ips[i])
		}

		// Read the file
		content, err := os.ReadFile(blocklistPath)
		if err != nil {
			t.Fatalf("failed to read blocklist file: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(string(content)), "\n")

		// Property: File MUST contain exactly numIPs lines, each in IP/32 format
		if len(lines) != numIPs {
			t.Fatalf("expected %d lines in blocklist, got %d: %q", numIPs, len(lines), string(content))
		}

		for i, ip := range ips {
			expected := ip + "/32"
			if strings.TrimSpace(lines[i]) != expected {
				t.Fatalf("expected line %d to be %q, got %q", i, expected, lines[i])
			}
		}
	})
}
