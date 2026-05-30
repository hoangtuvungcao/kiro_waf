// Feature: waf-system-overhaul, Property 13: L7 Ban to XDP Sync
// **Validates: Requirements 6.5**
//
// For any IP address vượt ngưỡng `hard_block_after` requests trong một phút,
// IP đó SHALL xuất hiện trong cả L7 BanStore (in-memory) và file blocklist XDP
// sau khi ban được thực thi.
package property

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kiro_waf/internal/client/ban"

	"pgregory.net/rapid"
)

// TestBanSync verifies that for any IP address, after Ban() is called,
// the IP appears in both the L7 BanStore (in-memory) and the XDP blocklist file.
func TestBanSync(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random valid IPv4 address
		octet1 := rapid.IntRange(1, 223).Draw(t, "octet1")
		octet2 := rapid.IntRange(0, 255).Draw(t, "octet2")
		octet3 := rapid.IntRange(0, 255).Draw(t, "octet3")
		octet4 := rapid.IntRange(1, 254).Draw(t, "octet4")

		ip := fmt.Sprintf("%d.%d.%d.%d", octet1, octet2, octet3, octet4)

		// Create a temp blocklist file
		tmpDir, err := os.MkdirTemp("", "ban_sync_test_*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		blocklistPath := filepath.Join(tmpDir, "xdp-blocklist.txt")

		// Create an InMemoryBanEngine with the temp file (no sync command needed for test)
		engine := ban.NewInMemoryBanEngine(blocklistPath, "")

		// Ban the IP with a reasonable duration and reason
		duration := time.Duration(rapid.IntRange(1, 60).Draw(t, "duration_minutes")) * time.Minute
		reason := rapid.StringMatching(`[a-zA-Z0-9 ]{5,30}`).Draw(t, "reason")

		engine.Ban(ip, duration, reason)

		// Verify: IsBanned(ip) returns true (L7 store)
		if !engine.IsBanned(ip) {
			t.Fatalf("L7 BanStore: IsBanned(%q) returned false after Ban() was called", ip)
		}

		// Verify: blocklist file contains the IP/32 entry (XDP sync)
		content, err := os.ReadFile(blocklistPath)
		if err != nil {
			t.Fatalf("failed to read blocklist file: %v", err)
		}

		expectedEntry := ip + "/32"
		if !strings.Contains(string(content), expectedEntry) {
			t.Fatalf("XDP blocklist file does not contain %q. File content: %q", expectedEntry, string(content))
		}
	})
}
