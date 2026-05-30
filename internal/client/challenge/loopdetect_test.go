package challenge

import (
	"testing"
	"time"
)

func TestLoopDetector_ShouldBypass_NoRecords(t *testing.T) {
	ld := NewLoopDetector()
	if ld.ShouldBypass("192.168.1.1", "transparent") {
		t.Error("expected ShouldBypass to return false with no records")
	}
}

func TestLoopDetector_ShouldBypass_BelowThreshold(t *testing.T) {
	ld := NewLoopDetector()

	// Record 3 times — should NOT trigger bypass (threshold is >3)
	ld.Record("192.168.1.1", "transparent")
	ld.Record("192.168.1.1", "transparent")
	ld.Record("192.168.1.1", "transparent")

	if ld.ShouldBypass("192.168.1.1", "transparent") {
		t.Error("expected ShouldBypass to return false with exactly 3 records")
	}
}

func TestLoopDetector_ShouldBypass_AboveThreshold(t *testing.T) {
	ld := NewLoopDetector()

	// Record 4 times — should trigger bypass (>3)
	ld.Record("192.168.1.1", "transparent")
	ld.Record("192.168.1.1", "transparent")
	ld.Record("192.168.1.1", "transparent")
	ld.Record("192.168.1.1", "transparent")

	if !ld.ShouldBypass("192.168.1.1", "transparent") {
		t.Error("expected ShouldBypass to return true with 4 records")
	}
}

func TestLoopDetector_ShouldBypass_DifferentChallengeTypes(t *testing.T) {
	ld := NewLoopDetector()

	// Record 4 times for "transparent"
	ld.Record("192.168.1.1", "transparent")
	ld.Record("192.168.1.1", "transparent")
	ld.Record("192.168.1.1", "transparent")
	ld.Record("192.168.1.1", "transparent")

	// Different challenge type should not be affected
	if ld.ShouldBypass("192.168.1.1", "pow") {
		t.Error("expected ShouldBypass to return false for different challenge type")
	}
}

func TestLoopDetector_ShouldBypass_DifferentIPs(t *testing.T) {
	ld := NewLoopDetector()

	// Record 4 times for one IP
	ld.Record("192.168.1.1", "transparent")
	ld.Record("192.168.1.1", "transparent")
	ld.Record("192.168.1.1", "transparent")
	ld.Record("192.168.1.1", "transparent")

	// Different IP should not be affected
	if ld.ShouldBypass("192.168.1.2", "transparent") {
		t.Error("expected ShouldBypass to return false for different IP")
	}
}

func TestLoopDetector_Record_PrunesOldTimestamps(t *testing.T) {
	ld := NewLoopDetector()

	// Manually insert old timestamps
	key := "10.0.0.1:transparent"
	ld.mu.Lock()
	ld.records[key] = &loopRecord{
		timestamps: []time.Time{
			time.Now().Add(-15 * time.Second),
			time.Now().Add(-12 * time.Second),
			time.Now().Add(-11 * time.Second),
		},
	}
	ld.mu.Unlock()

	// Record a new one — old ones should be pruned
	ld.Record("10.0.0.1", "transparent")

	ld.mu.Lock()
	rec := ld.records[key]
	if len(rec.timestamps) != 1 {
		t.Errorf("expected 1 timestamp after pruning, got %d", len(rec.timestamps))
	}
	ld.mu.Unlock()
}

func TestLoopDetector_Cleanup_RemovesOldEntries(t *testing.T) {
	ld := NewLoopDetector()

	// Insert entries with old timestamps (>30s ago)
	ld.mu.Lock()
	ld.records["old-ip:transparent"] = &loopRecord{
		timestamps: []time.Time{
			time.Now().Add(-60 * time.Second),
			time.Now().Add(-45 * time.Second),
		},
	}
	ld.records["recent-ip:pow"] = &loopRecord{
		timestamps: []time.Time{
			time.Now().Add(-5 * time.Second),
		},
	}
	ld.mu.Unlock()

	ld.Cleanup()

	ld.mu.Lock()
	defer ld.mu.Unlock()

	if _, exists := ld.records["old-ip:transparent"]; exists {
		t.Error("expected old entry to be removed by Cleanup")
	}
	if _, exists := ld.records["recent-ip:pow"]; !exists {
		t.Error("expected recent entry to be preserved by Cleanup")
	}
}

func TestLoopDetector_Cleanup_PreservesPartiallyFreshEntries(t *testing.T) {
	ld := NewLoopDetector()

	// Entry with mix of old and recent timestamps
	ld.mu.Lock()
	ld.records["mixed-ip:hold"] = &loopRecord{
		timestamps: []time.Time{
			time.Now().Add(-60 * time.Second), // old
			time.Now().Add(-5 * time.Second),  // recent
		},
	}
	ld.mu.Unlock()

	ld.Cleanup()

	ld.mu.Lock()
	defer ld.mu.Unlock()

	if _, exists := ld.records["mixed-ip:hold"]; !exists {
		t.Error("expected entry with at least one recent timestamp to be preserved")
	}
}
