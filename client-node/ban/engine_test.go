package ban

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewInMemoryBanEngine(t *testing.T) {
	engine := NewInMemoryBanEngine("/tmp/blocklist.txt", "echo sync")
	if engine == nil {
		t.Fatal("NewInMemoryBanEngine returned nil")
	}
	if engine.blocklistPath != "/tmp/blocklist.txt" {
		t.Errorf("expected blocklistPath=/tmp/blocklist.txt, got %s", engine.blocklistPath)
	}
	if engine.syncCommand != "echo sync" {
		t.Errorf("expected syncCommand='echo sync', got %s", engine.syncCommand)
	}
	if engine.store == nil {
		t.Error("expected store to be initialized")
	}
}

func TestIsBanned_NotBanned(t *testing.T) {
	engine := NewInMemoryBanEngine("", "")
	if engine.IsBanned("1.2.3.4") {
		t.Error("expected IsBanned=false for unknown IP")
	}
}

func TestIsBanned_ActiveBan(t *testing.T) {
	engine := NewInMemoryBanEngine("", "")
	now := time.Now()
	engine.nowFunc = func() time.Time { return now }

	engine.store["10.0.0.1"] = BanEntry{
		IP:        "10.0.0.1",
		ExpiresAt: now.Add(5 * time.Minute),
		Reason:    "rate limit exceeded",
	}

	if !engine.IsBanned("10.0.0.1") {
		t.Error("expected IsBanned=true for active ban")
	}
}

func TestIsBanned_ExpiredBan(t *testing.T) {
	engine := NewInMemoryBanEngine("", "")
	now := time.Now()
	engine.nowFunc = func() time.Time { return now }

	engine.store["10.0.0.1"] = BanEntry{
		IP:        "10.0.0.1",
		ExpiresAt: now.Add(-1 * time.Minute), // already expired
		Reason:    "rate limit exceeded",
	}

	if engine.IsBanned("10.0.0.1") {
		t.Error("expected IsBanned=false for expired ban")
	}

	// Verify expired entry was cleaned up
	engine.mu.RLock()
	_, exists := engine.store["10.0.0.1"]
	engine.mu.RUnlock()
	if exists {
		t.Error("expected expired entry to be removed from store")
	}
}

func TestBan_AddsToStore(t *testing.T) {
	tmpDir := t.TempDir()
	blocklistPath := filepath.Join(tmpDir, "blocklist.txt")

	engine := NewInMemoryBanEngine(blocklistPath, "")
	now := time.Now()
	engine.nowFunc = func() time.Time { return now }

	engine.Ban("192.168.1.100", 10*time.Minute, "hard block threshold")

	if !engine.IsBanned("192.168.1.100") {
		t.Error("expected IP to be banned after Ban()")
	}

	entry, exists := engine.GetBanEntry("192.168.1.100")
	if !exists {
		t.Fatal("expected ban entry to exist")
	}
	if entry.IP != "192.168.1.100" {
		t.Errorf("expected IP=192.168.1.100, got %s", entry.IP)
	}
	if entry.Reason != "hard block threshold" {
		t.Errorf("expected reason='hard block threshold', got %s", entry.Reason)
	}
	expectedExpiry := now.Add(10 * time.Minute)
	if !entry.ExpiresAt.Equal(expectedExpiry) {
		t.Errorf("expected ExpiresAt=%v, got %v", expectedExpiry, entry.ExpiresAt)
	}
}

func TestBan_AppendsToBlocklistFile(t *testing.T) {
	tmpDir := t.TempDir()
	blocklistPath := filepath.Join(tmpDir, "blocklist.txt")

	engine := NewInMemoryBanEngine(blocklistPath, "")

	engine.Ban("10.20.30.40", 5*time.Minute, "test ban")
	engine.Ban("50.60.70.80", 5*time.Minute, "test ban 2")

	content, err := os.ReadFile(blocklistPath)
	if err != nil {
		t.Fatalf("failed to read blocklist file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines in blocklist, got %d: %q", len(lines), string(content))
	}
	if lines[0] != "10.20.30.40/32" {
		t.Errorf("expected first line='10.20.30.40/32', got %q", lines[0])
	}
	if lines[1] != "50.60.70.80/32" {
		t.Errorf("expected second line='50.60.70.80/32', got %q", lines[1])
	}
}

func TestBan_BlocklistFileNotWritable(t *testing.T) {
	// Use a path that doesn't exist and can't be created
	engine := NewInMemoryBanEngine("/nonexistent/dir/blocklist.txt", "")

	// Should not panic - graceful degradation
	engine.Ban("1.2.3.4", 5*time.Minute, "test")

	// IP should still be in memory store even if file write failed
	if !engine.IsBanned("1.2.3.4") {
		t.Error("expected IP to be banned in memory even when file write fails")
	}
}

func TestBan_TriggersSyncCommand(t *testing.T) {
	tmpDir := t.TempDir()
	blocklistPath := filepath.Join(tmpDir, "blocklist.txt")
	markerFile := filepath.Join(tmpDir, "sync_called")

	// Use a command that creates a marker file to prove it was called
	syncCmd := "touch " + markerFile

	engine := NewInMemoryBanEngine(blocklistPath, syncCmd)
	engine.Ban("1.1.1.1", 5*time.Minute, "test sync")

	// Check that the sync command was executed
	if _, err := os.Stat(markerFile); os.IsNotExist(err) {
		t.Error("expected sync command to be executed (marker file not found)")
	}
}

func TestUnban(t *testing.T) {
	engine := NewInMemoryBanEngine("", "")
	now := time.Now()
	engine.nowFunc = func() time.Time { return now }

	engine.store["10.0.0.1"] = BanEntry{
		IP:        "10.0.0.1",
		ExpiresAt: now.Add(5 * time.Minute),
		Reason:    "test",
	}

	if !engine.IsBanned("10.0.0.1") {
		t.Fatal("expected IP to be banned before Unban()")
	}

	engine.Unban("10.0.0.1")

	if engine.IsBanned("10.0.0.1") {
		t.Error("expected IP to not be banned after Unban()")
	}
}

func TestUnban_NonExistentIP(t *testing.T) {
	engine := NewInMemoryBanEngine("", "")
	// Should not panic
	engine.Unban("1.2.3.4")
}

func TestSyncToXDP_EmptyCommand(t *testing.T) {
	engine := NewInMemoryBanEngine("", "")
	err := engine.SyncToXDP()
	if err != nil {
		t.Errorf("expected no error for empty sync command, got %v", err)
	}
}

func TestSyncToXDP_ValidCommand(t *testing.T) {
	engine := NewInMemoryBanEngine("", "echo hello")
	err := engine.SyncToXDP()
	if err != nil {
		t.Errorf("expected no error for valid sync command, got %v", err)
	}
}

func TestSyncToXDP_InvalidCommand(t *testing.T) {
	engine := NewInMemoryBanEngine("", "/nonexistent/command")
	err := engine.SyncToXDP()
	if err == nil {
		t.Error("expected error for invalid sync command")
	}
}

func TestCleanupExpired(t *testing.T) {
	engine := NewInMemoryBanEngine("", "")
	now := time.Now()
	engine.nowFunc = func() time.Time { return now }

	// Add mix of expired and active bans
	engine.store["expired1"] = BanEntry{
		IP:        "expired1",
		ExpiresAt: now.Add(-10 * time.Minute),
		Reason:    "old ban",
	}
	engine.store["expired2"] = BanEntry{
		IP:        "expired2",
		ExpiresAt: now.Add(-1 * time.Second),
		Reason:    "just expired",
	}
	engine.store["active1"] = BanEntry{
		IP:        "active1",
		ExpiresAt: now.Add(5 * time.Minute),
		Reason:    "still active",
	}

	engine.CleanupExpired()

	engine.mu.RLock()
	defer engine.mu.RUnlock()

	if _, exists := engine.store["expired1"]; exists {
		t.Error("expected expired1 to be cleaned up")
	}
	if _, exists := engine.store["expired2"]; exists {
		t.Error("expected expired2 to be cleaned up")
	}
	if _, exists := engine.store["active1"]; !exists {
		t.Error("expected active1 to still exist")
	}
}

func TestGetBanEntry_Exists(t *testing.T) {
	engine := NewInMemoryBanEngine("", "")
	now := time.Now()
	engine.nowFunc = func() time.Time { return now }

	engine.store["10.0.0.1"] = BanEntry{
		IP:        "10.0.0.1",
		ExpiresAt: now.Add(5 * time.Minute),
		Reason:    "test reason",
	}

	entry, exists := engine.GetBanEntry("10.0.0.1")
	if !exists {
		t.Fatal("expected entry to exist")
	}
	if entry.Reason != "test reason" {
		t.Errorf("expected reason='test reason', got %s", entry.Reason)
	}
}

func TestGetBanEntry_NotExists(t *testing.T) {
	engine := NewInMemoryBanEngine("", "")
	_, exists := engine.GetBanEntry("1.2.3.4")
	if exists {
		t.Error("expected entry to not exist")
	}
}

func TestGetBanEntry_Expired(t *testing.T) {
	engine := NewInMemoryBanEngine("", "")
	now := time.Now()
	engine.nowFunc = func() time.Time { return now }

	engine.store["10.0.0.1"] = BanEntry{
		IP:        "10.0.0.1",
		ExpiresAt: now.Add(-1 * time.Minute),
		Reason:    "expired",
	}

	_, exists := engine.GetBanEntry("10.0.0.1")
	if exists {
		t.Error("expected expired entry to not be returned")
	}
}

func TestBannedCount(t *testing.T) {
	engine := NewInMemoryBanEngine("", "")
	now := time.Now()
	engine.nowFunc = func() time.Time { return now }

	engine.store["active1"] = BanEntry{
		IP:        "active1",
		ExpiresAt: now.Add(5 * time.Minute),
		Reason:    "active",
	}
	engine.store["active2"] = BanEntry{
		IP:        "active2",
		ExpiresAt: now.Add(10 * time.Minute),
		Reason:    "active",
	}
	engine.store["expired1"] = BanEntry{
		IP:        "expired1",
		ExpiresAt: now.Add(-1 * time.Minute),
		Reason:    "expired",
	}

	count := engine.BannedCount()
	if count != 2 {
		t.Errorf("expected BannedCount=2, got %d", count)
	}
}

func TestConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	blocklistPath := filepath.Join(tmpDir, "blocklist.txt")

	engine := NewInMemoryBanEngine(blocklistPath, "")

	var wg sync.WaitGroup
	numGoroutines := 10
	opsPerGoroutine := 50

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ip := "10.0.0." + itoa(id+1)
			for i := 0; i < opsPerGoroutine; i++ {
				engine.Ban(ip, 5*time.Minute, "concurrent test")
				engine.IsBanned(ip)
				engine.GetBanEntry(ip)
				engine.BannedCount()
				if i%10 == 0 {
					engine.Unban(ip)
				}
			}
		}(g)
	}

	wg.Wait()
	// If we reach here without panic or deadlock, concurrency is safe
}

func TestBanOverwrite(t *testing.T) {
	engine := NewInMemoryBanEngine("", "")
	now := time.Now()
	engine.nowFunc = func() time.Time { return now }

	// Ban with short duration
	engine.Ban("10.0.0.1", 1*time.Minute, "first ban")

	// Ban again with longer duration
	engine.Ban("10.0.0.1", 1*time.Hour, "second ban")

	entry, exists := engine.GetBanEntry("10.0.0.1")
	if !exists {
		t.Fatal("expected entry to exist")
	}
	if entry.Reason != "second ban" {
		t.Errorf("expected reason='second ban', got %s", entry.Reason)
	}
	expectedExpiry := now.Add(1 * time.Hour)
	if !entry.ExpiresAt.Equal(expectedExpiry) {
		t.Errorf("expected ExpiresAt=%v, got %v", expectedExpiry, entry.ExpiresAt)
	}
}

func TestImplementsBanEngineInterface(t *testing.T) {
	engine := NewInMemoryBanEngine("", "")
	// Verify that InMemoryBanEngine implements BanEngine interface
	var _ BanEngine = engine
}

// itoa converts an int to a string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}
