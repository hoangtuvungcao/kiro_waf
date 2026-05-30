package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompressDecompressGzip(t *testing.T) {
	original := []byte(`{"timestamp":"2024-01-01T00:00:00Z","rate_limit_state":{},"session_state":{},"ban_list":[],"xdp_stats":{}}`)

	compressed, err := compressGzip(original)
	if err != nil {
		t.Fatalf("compressGzip failed: %v", err)
	}

	// Compressed should be smaller or at least valid gzip
	if len(compressed) == 0 {
		t.Fatal("compressed data is empty")
	}

	decompressed, err := DecompressGzip(compressed)
	if err != nil {
		t.Fatalf("DecompressGzip failed: %v", err)
	}

	if string(decompressed) != string(original) {
		t.Errorf("round-trip failed: got %s, want %s", decompressed, original)
	}
}

func TestSnapshotWriterWritesSnapshot(t *testing.T) {
	var writtenPath string
	var writtenData []byte

	now := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	provider := func() HealthSnapshot {
		return HealthSnapshot{
			RateLimitState: map[string]RateBucket{
				"192.168.1.1": {IP: "192.168.1.1", Count: 10, WindowEnd: now.Add(60 * time.Second)},
			},
			SessionState: map[string]SessionData{
				"sess1": {Token: "abc123", CreatedAt: now, ExpiresAt: now.Add(time.Hour)},
			},
			BanList: []BanEntry{
				{IP: "10.0.0.1", ExpiresAt: now.Add(time.Hour), Reason: "rate_limit"},
			},
			XDPStats: XDPStats{TotalPackets: 1000, PassedPackets: 900, DroppedPackets: 100},
		}
	}

	config := MonitorConfig{
		SnapshotInterval: 1 * time.Second,
		SnapshotMaxSize:  64 * 1024 * 1024,
		SnapshotPath:     "/tmp/kiro-test-snapshots",
	}

	sw := NewSnapshotWriter(config, provider,
		WithSnapshotNowFunc(func() time.Time { return now }),
		WithSnapshotWriteFileFunc(func(path string, data []byte) error {
			writtenPath = path
			writtenData = data
			return nil
		}),
	)

	// Manually trigger a snapshot write
	sw.writeSnapshot()

	// Verify the snapshot was written
	if writtenPath == "" {
		t.Fatal("snapshot was not written")
	}

	expectedPath := filepath.Join("/tmp/kiro-test-snapshots", "state.snapshot.gz")
	if writtenPath != expectedPath {
		t.Errorf("unexpected path: got %s, want %s", writtenPath, expectedPath)
	}

	// Decompress and verify content
	decompressed, err := DecompressGzip(writtenData)
	if err != nil {
		t.Fatalf("failed to decompress snapshot: %v", err)
	}

	var snapshot HealthSnapshot
	if err := json.Unmarshal(decompressed, &snapshot); err != nil {
		t.Fatalf("failed to unmarshal snapshot: %v", err)
	}

	if snapshot.Timestamp != now {
		t.Errorf("timestamp mismatch: got %v, want %v", snapshot.Timestamp, now)
	}

	if len(snapshot.RateLimitState) != 1 {
		t.Errorf("rate limit state count: got %d, want 1", len(snapshot.RateLimitState))
	}

	if len(snapshot.SessionState) != 1 {
		t.Errorf("session state count: got %d, want 1", len(snapshot.SessionState))
	}

	if len(snapshot.BanList) != 1 {
		t.Errorf("ban list count: got %d, want 1", len(snapshot.BanList))
	}

	if snapshot.XDPStats.TotalPackets != 1000 {
		t.Errorf("xdp total packets: got %d, want 1000", snapshot.XDPStats.TotalPackets)
	}
}

func TestSnapshotWriterIOError(t *testing.T) {
	// Requirement 13.13: On I/O failure, log warning, keep old snapshot, retry next cycle
	now := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	writeAttempts := 0

	provider := func() HealthSnapshot {
		return HealthSnapshot{
			RateLimitState: map[string]RateBucket{},
			SessionState:   map[string]SessionData{},
			BanList:        []BanEntry{},
			XDPStats:       XDPStats{},
		}
	}

	config := MonitorConfig{
		SnapshotInterval: 1 * time.Second,
		SnapshotMaxSize:  64 * 1024 * 1024,
		SnapshotPath:     "/tmp/kiro-test-snapshots",
	}

	sw := NewSnapshotWriter(config, provider,
		WithSnapshotNowFunc(func() time.Time { return now }),
		WithSnapshotWriteFileFunc(func(path string, data []byte) error {
			writeAttempts++
			if writeAttempts == 1 {
				return errors.New("disk full")
			}
			return nil
		}),
	)

	// First write should fail
	sw.writeSnapshot()
	if sw.LastSnapshotTime() != (time.Time{}) {
		t.Error("lastSnapshot should be zero after failed write")
	}

	// Second write should succeed
	sw.writeSnapshot()
	if sw.LastSnapshotTime() != now {
		t.Errorf("lastSnapshot should be %v after successful write, got %v", now, sw.LastSnapshotTime())
	}

	if writeAttempts != 2 {
		t.Errorf("expected 2 write attempts, got %d", writeAttempts)
	}
}

func TestSnapshotWriterSizeLimit(t *testing.T) {
	// Requirement 13.8: Max 64MB, truncate oldest entries if exceeded
	now := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	// Create a large state that exceeds a small size limit
	largeState := HealthSnapshot{
		RateLimitState: make(map[string]RateBucket),
		SessionState:   make(map[string]SessionData),
		BanList:        make([]BanEntry, 0),
		XDPStats:       XDPStats{TotalPackets: 1000},
	}

	// Add many entries to exceed the size limit
	for i := 0; i < 1000; i++ {
		ip := fmt.Sprintf("192.168.%d.%d", i/256, i%256)
		largeState.RateLimitState[ip] = RateBucket{
			IP:        ip,
			Count:     i,
			WindowEnd: now.Add(time.Duration(i) * time.Second),
		}
		largeState.SessionState[fmt.Sprintf("sess_%d", i)] = SessionData{
			Token:     fmt.Sprintf("token_%d_%s", i, strings.Repeat("x", 100)),
			CreatedAt: now.Add(time.Duration(i) * time.Second),
			ExpiresAt: now.Add(time.Duration(i+3600) * time.Second),
		}
		largeState.BanList = append(largeState.BanList, BanEntry{
			IP:        ip,
			ExpiresAt: now.Add(time.Duration(i) * time.Second),
			Reason:    "test_ban",
		})
	}

	var writtenData []byte

	// Use a very small max size to force truncation
	config := MonitorConfig{
		SnapshotInterval: 1 * time.Second,
		SnapshotMaxSize:  4 * 1024, // 4KB to force truncation
		SnapshotPath:     "/tmp/kiro-test-snapshots",
	}

	sw := NewSnapshotWriter(config, func() HealthSnapshot { return largeState },
		WithSnapshotNowFunc(func() time.Time { return now }),
		WithSnapshotWriteFileFunc(func(path string, data []byte) error {
			writtenData = data
			return nil
		}),
	)

	sw.writeSnapshot()

	if writtenData == nil {
		t.Fatal("snapshot was not written after truncation")
	}

	// Verify the written data is within size limit
	if int64(len(writtenData)) > config.SnapshotMaxSize {
		t.Errorf("snapshot size %d exceeds max %d", len(writtenData), config.SnapshotMaxSize)
	}

	// Verify it's valid gzip
	decompressed, err := DecompressGzip(writtenData)
	if err != nil {
		t.Fatalf("failed to decompress truncated snapshot: %v", err)
	}

	// Verify it's valid JSON
	var snapshot HealthSnapshot
	if err := json.Unmarshal(decompressed, &snapshot); err != nil {
		t.Fatalf("failed to unmarshal truncated snapshot: %v", err)
	}

	// Verify entries were truncated (should have fewer than original 1000)
	totalEntries := len(snapshot.RateLimitState) + len(snapshot.SessionState) + len(snapshot.BanList)
	if totalEntries >= 3000 {
		t.Errorf("expected truncation to reduce entries, got %d total", totalEntries)
	}
}

func TestSnapshotWriterStartStop(t *testing.T) {
	writeCount := 0

	provider := func() HealthSnapshot {
		return HealthSnapshot{
			RateLimitState: map[string]RateBucket{},
			SessionState:   map[string]SessionData{},
			BanList:        []BanEntry{},
			XDPStats:       XDPStats{},
		}
	}

	config := MonitorConfig{
		SnapshotInterval: 50 * time.Millisecond, // Fast interval for testing
		SnapshotMaxSize:  64 * 1024 * 1024,
		SnapshotPath:     "/tmp/kiro-test-snapshots",
	}

	sw := NewSnapshotWriter(config, provider,
		WithSnapshotWriteFileFunc(func(path string, data []byte) error {
			writeCount++
			return nil
		}),
	)

	ctx := context.Background()
	sw.Start(ctx)

	// Wait for a few snapshot cycles
	time.Sleep(200 * time.Millisecond)

	sw.Stop()

	if writeCount == 0 {
		t.Error("expected at least one snapshot write during the test period")
	}
}

func TestSnapshotWriterAtomicWrite(t *testing.T) {
	// Test that the actual file write uses atomic pattern (temp + rename)
	tmpDir := t.TempDir()

	provider := func() HealthSnapshot {
		return HealthSnapshot{
			RateLimitState: map[string]RateBucket{
				"1.2.3.4": {IP: "1.2.3.4", Count: 5, WindowEnd: time.Now().Add(time.Minute)},
			},
			SessionState: map[string]SessionData{},
			BanList:      []BanEntry{},
			XDPStats:     XDPStats{TotalPackets: 42},
		}
	}

	config := MonitorConfig{
		SnapshotInterval: 1 * time.Second,
		SnapshotMaxSize:  64 * 1024 * 1024,
		SnapshotPath:     tmpDir,
	}

	sw := NewSnapshotWriter(config, provider)

	sw.writeSnapshot()

	// Verify the file exists
	snapshotPath := filepath.Join(tmpDir, "state.snapshot.gz")
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("failed to read snapshot file: %v", err)
	}

	// Verify it's valid gzip
	decompressed, err := DecompressGzip(data)
	if err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}

	var snapshot HealthSnapshot
	if err := json.Unmarshal(decompressed, &snapshot); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if snapshot.XDPStats.TotalPackets != 42 {
		t.Errorf("expected TotalPackets=42, got %d", snapshot.XDPStats.TotalPackets)
	}

	// Verify no temp file remains
	tmpPath := snapshotPath + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temp file should not exist after successful write")
	}
}

func TestTruncateRateLimitState(t *testing.T) {
	now := time.Now()
	state := map[string]RateBucket{
		"ip1": {IP: "ip1", Count: 1, WindowEnd: now.Add(1 * time.Second)},
		"ip2": {IP: "ip2", Count: 2, WindowEnd: now.Add(2 * time.Second)},
		"ip3": {IP: "ip3", Count: 3, WindowEnd: now.Add(3 * time.Second)},
		"ip4": {IP: "ip4", Count: 4, WindowEnd: now.Add(4 * time.Second)},
	}

	result := truncateRateLimitState(state)

	// Should keep the newest half (2 entries)
	if len(result) != 2 {
		t.Errorf("expected 2 entries after truncation, got %d", len(result))
	}

	// The newest entries (ip3, ip4) should be kept
	if _, ok := result["ip3"]; !ok {
		t.Error("expected ip3 to be kept (newer entry)")
	}
	if _, ok := result["ip4"]; !ok {
		t.Error("expected ip4 to be kept (newer entry)")
	}
}

func TestTruncateSessionState(t *testing.T) {
	now := time.Now()
	state := map[string]SessionData{
		"s1": {Token: "t1", CreatedAt: now.Add(1 * time.Second), ExpiresAt: now.Add(time.Hour)},
		"s2": {Token: "t2", CreatedAt: now.Add(2 * time.Second), ExpiresAt: now.Add(time.Hour)},
		"s3": {Token: "t3", CreatedAt: now.Add(3 * time.Second), ExpiresAt: now.Add(time.Hour)},
		"s4": {Token: "t4", CreatedAt: now.Add(4 * time.Second), ExpiresAt: now.Add(time.Hour)},
	}

	result := truncateSessionState(state)

	// Should keep the newest half (2 entries)
	if len(result) != 2 {
		t.Errorf("expected 2 entries after truncation, got %d", len(result))
	}

	// The newest entries (s3, s4) should be kept
	if _, ok := result["s3"]; !ok {
		t.Error("expected s3 to be kept (newer entry)")
	}
	if _, ok := result["s4"]; !ok {
		t.Error("expected s4 to be kept (newer entry)")
	}
}

func TestTruncateBanList(t *testing.T) {
	now := time.Now()
	list := []BanEntry{
		{IP: "1.1.1.1", ExpiresAt: now.Add(1 * time.Second), Reason: "r1"},
		{IP: "2.2.2.2", ExpiresAt: now.Add(2 * time.Second), Reason: "r2"},
		{IP: "3.3.3.3", ExpiresAt: now.Add(3 * time.Second), Reason: "r3"},
		{IP: "4.4.4.4", ExpiresAt: now.Add(4 * time.Second), Reason: "r4"},
	}

	result := truncateBanList(list)

	// Should keep the newest half (2 entries)
	if len(result) != 2 {
		t.Errorf("expected 2 entries after truncation, got %d", len(result))
	}

	// The newest entries should be kept (sorted by ExpiresAt, keep newest)
	for _, entry := range result {
		if entry.IP == "1.1.1.1" || entry.IP == "2.2.2.2" {
			t.Errorf("expected oldest entries to be removed, but found %s", entry.IP)
		}
	}
}

func TestTruncateEmptyCollections(t *testing.T) {
	// Truncating empty or single-element collections
	emptyRate := truncateRateLimitState(map[string]RateBucket{})
	if len(emptyRate) != 0 {
		t.Errorf("expected empty result for empty input, got %d", len(emptyRate))
	}

	singleRate := truncateRateLimitState(map[string]RateBucket{
		"ip1": {IP: "ip1", Count: 1, WindowEnd: time.Now()},
	})
	if len(singleRate) != 0 {
		t.Errorf("expected empty result for single entry, got %d", len(singleRate))
	}

	emptySession := truncateSessionState(map[string]SessionData{})
	if len(emptySession) != 0 {
		t.Errorf("expected empty result for empty input, got %d", len(emptySession))
	}

	emptyBan := truncateBanList([]BanEntry{})
	if emptyBan != nil {
		t.Errorf("expected nil for empty ban list, got %v", emptyBan)
	}

	singleBan := truncateBanList([]BanEntry{{IP: "1.1.1.1", ExpiresAt: time.Now()}})
	if singleBan != nil {
		t.Errorf("expected nil for single ban entry, got %v", singleBan)
	}
}

func TestDecompressGzipInvalidData(t *testing.T) {
	_, err := DecompressGzip([]byte("not gzip data"))
	if err == nil {
		t.Error("expected error for invalid gzip data")
	}
}

func TestCompressGzipProducesValidGzip(t *testing.T) {
	data := []byte(`{"test": "data", "number": 42}`)

	compressed, err := compressGzip(data)
	if err != nil {
		t.Fatalf("compressGzip failed: %v", err)
	}

	// Verify it's valid gzip by checking magic bytes (0x1f, 0x8b)
	if len(compressed) < 3 || compressed[0] != 0x1f || compressed[1] != 0x8b {
		t.Error("compressed data does not have gzip magic bytes")
	}

	// Verify gzip method byte is deflate (8)
	if compressed[2] != 8 {
		t.Errorf("unexpected gzip method: got %d, want 8 (deflate)", compressed[2])
	}
}

func TestSnapshotWriterEmptyState(t *testing.T) {
	var writtenData []byte

	provider := func() HealthSnapshot {
		return HealthSnapshot{
			RateLimitState: map[string]RateBucket{},
			SessionState:   map[string]SessionData{},
			BanList:        []BanEntry{},
			XDPStats:       XDPStats{},
		}
	}

	config := MonitorConfig{
		SnapshotInterval: 1 * time.Second,
		SnapshotMaxSize:  64 * 1024 * 1024,
		SnapshotPath:     "/tmp/kiro-test-snapshots",
	}

	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	sw := NewSnapshotWriter(config, provider,
		WithSnapshotNowFunc(func() time.Time { return now }),
		WithSnapshotWriteFileFunc(func(path string, data []byte) error {
			writtenData = data
			return nil
		}),
	)

	sw.writeSnapshot()

	if writtenData == nil {
		t.Fatal("snapshot was not written for empty state")
	}

	decompressed, err := DecompressGzip(writtenData)
	if err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}

	var snapshot HealthSnapshot
	if err := json.Unmarshal(decompressed, &snapshot); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if snapshot.Timestamp != now {
		t.Errorf("timestamp mismatch: got %v, want %v", snapshot.Timestamp, now)
	}
}
