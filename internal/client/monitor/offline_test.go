package monitor

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestOfflineManager_TransitionToOffline(t *testing.T) {
	baseTime := time.Now()
	currentTime := baseTime

	cfg := DefaultConfig()
	cfg.OfflineThreshold = 60 * time.Second
	cfg.ReconnectInterval = 30 * time.Second
	cfg.MaxReconnectBackoff = 5 * time.Minute

	var modeChanges []OperationMode

	om := NewOfflineManager(cfg, nil,
		WithNowFunc(func() time.Time { return currentTime }),
		WithOnModeChange(func(mode OperationMode) {
			modeChanges = append(modeChanges, mode)
		}),
	)

	// Initially should not be offline
	if om.CheckOfflineStatus() {
		t.Fatal("expected online initially")
	}

	// Advance time by 30 seconds - should still be online
	currentTime = baseTime.Add(30 * time.Second)
	transitioned := om.EvaluateConnectivity()
	if transitioned {
		t.Fatal("should not transition to offline before threshold")
	}
	if om.CheckOfflineStatus() {
		t.Fatal("expected online before threshold")
	}

	// Advance time past the 60-second threshold
	currentTime = baseTime.Add(61 * time.Second)
	transitioned = om.EvaluateConnectivity()
	if !transitioned {
		t.Fatal("should transition to offline after threshold")
	}
	if !om.CheckOfflineStatus() {
		t.Fatal("expected offline after threshold")
	}

	// Verify mode change callback was called
	if len(modeChanges) != 1 || modeChanges[0] != ModeOffline {
		t.Fatalf("expected mode change to offline, got %v", modeChanges)
	}

	// Calling EvaluateConnectivity again should not transition again
	transitioned = om.EvaluateConnectivity()
	if transitioned {
		t.Fatal("should not transition again when already offline")
	}
}

func TestOfflineManager_ExponentialBackoff(t *testing.T) {
	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 30 * time.Second},   // 30s * 2^0 = 30s
		{2, 60 * time.Second},   // 30s * 2^1 = 60s
		{3, 120 * time.Second},  // 30s * 2^2 = 120s
		{4, 240 * time.Second},  // 30s * 2^3 = 240s
		{5, 300 * time.Second},  // 30s * 2^4 = 480s → capped at 300s (5 min)
		{6, 300 * time.Second},  // Still capped at 300s
		{10, 300 * time.Second}, // Still capped at 300s
	}

	baseInterval := 30 * time.Second
	maxBackoff := 5 * time.Minute

	for _, tt := range tests {
		result := CalculateBackoff(baseInterval, maxBackoff, tt.attempt)
		if result != tt.expected {
			t.Errorf("CalculateBackoff(attempt=%d): got %s, want %s",
				tt.attempt, result, tt.expected)
		}
	}
}

func TestOfflineManager_ReconnectSuccess(t *testing.T) {
	baseTime := time.Now()
	currentTime := baseTime

	cfg := DefaultConfig()
	cfg.OfflineThreshold = 60 * time.Second
	cfg.ReconnectInterval = 30 * time.Second
	cfg.MaxReconnectBackoff = 5 * time.Minute

	var syncCalled atomic.Int32
	var modeChanges []OperationMode

	om := NewOfflineManager(cfg, nil,
		WithNowFunc(func() time.Time { return currentTime }),
		WithCheckMasterFunc(func(ctx context.Context) error {
			return nil // Success
		}),
		WithSyncConfigFunc(func(ctx context.Context) error {
			syncCalled.Add(1)
			return nil
		}),
		WithOnModeChange(func(mode OperationMode) {
			modeChanges = append(modeChanges, mode)
		}),
	)

	// Go offline
	currentTime = baseTime.Add(61 * time.Second)
	om.EvaluateConnectivity()
	if !om.CheckOfflineStatus() {
		t.Fatal("expected offline")
	}

	// Attempt reconnect - should succeed
	currentTime = baseTime.Add(92 * time.Second) // 61s + 31s (past first backoff)
	success := om.AttemptReconnect(context.Background())
	if !success {
		t.Fatal("expected reconnect to succeed")
	}

	// Should be back online
	if om.CheckOfflineStatus() {
		t.Fatal("expected online after successful reconnect")
	}

	// Verify sync was called
	if syncCalled.Load() != 1 {
		t.Fatalf("expected syncConfig to be called once, got %d", syncCalled.Load())
	}

	// Verify mode changes: offline → online
	if len(modeChanges) != 2 || modeChanges[0] != ModeOffline || modeChanges[1] != ModeOnline {
		t.Fatalf("expected mode changes [offline, online], got %v", modeChanges)
	}
}

func TestOfflineManager_ReconnectBackoffRespected(t *testing.T) {
	baseTime := time.Now()
	currentTime := baseTime

	cfg := DefaultConfig()
	cfg.OfflineThreshold = 60 * time.Second
	cfg.ReconnectInterval = 30 * time.Second
	cfg.MaxReconnectBackoff = 5 * time.Minute

	var checkCalls atomic.Int32

	om := NewOfflineManager(cfg, nil,
		WithNowFunc(func() time.Time { return currentTime }),
		WithCheckMasterFunc(func(ctx context.Context) error {
			checkCalls.Add(1)
			return context.DeadlineExceeded // Fail
		}),
	)

	// Go offline
	currentTime = baseTime.Add(61 * time.Second)
	om.EvaluateConnectivity()

	// First attempt (immediately after going offline, no previous attempt)
	om.AttemptReconnect(context.Background())
	if checkCalls.Load() != 1 {
		t.Fatalf("expected 1 check call, got %d", checkCalls.Load())
	}

	// After first failure, backoff doubles to 60s.
	// Try again only 10s later — should be blocked by backoff.
	currentTime = baseTime.Add(71 * time.Second)
	om.AttemptReconnect(context.Background())
	if checkCalls.Load() != 1 {
		t.Fatalf("expected still 1 check call (backoff not elapsed), got %d", checkCalls.Load())
	}

	// Try 59s after first attempt — still within 60s backoff
	currentTime = baseTime.Add(119 * time.Second)
	om.AttemptReconnect(context.Background())
	if checkCalls.Load() != 1 {
		t.Fatalf("expected still 1 check call (59s < 60s backoff), got %d", checkCalls.Load())
	}

	// Try 61s after first attempt — backoff elapsed (60s)
	currentTime = baseTime.Add(122 * time.Second)
	om.AttemptReconnect(context.Background())
	if checkCalls.Load() != 2 {
		t.Fatalf("expected 2 check calls after backoff elapsed, got %d", checkCalls.Load())
	}
}

func TestOfflineManager_RecordMasterContact(t *testing.T) {
	baseTime := time.Now()
	currentTime := baseTime

	cfg := DefaultConfig()
	cfg.OfflineThreshold = 60 * time.Second

	om := NewOfflineManager(cfg, nil,
		WithNowFunc(func() time.Time { return currentTime }),
	)

	// Record contact at t=50s resets the timer
	currentTime = baseTime.Add(50 * time.Second)
	om.RecordMasterContact(context.Background())

	// Now advance to t=100s (50s since last contact at t=50s)
	currentTime = baseTime.Add(100 * time.Second)
	transitioned := om.EvaluateConnectivity()
	if transitioned {
		t.Fatal("should not transition because last contact was only 50s ago")
	}

	// Advance to t=111s (61s since last contact at t=50s)
	currentTime = baseTime.Add(111 * time.Second)
	transitioned = om.EvaluateConnectivity()
	if !transitioned {
		t.Fatal("should transition to offline 61s after last contact")
	}
}

func TestCalculateBackoff_EdgeCases(t *testing.T) {
	base := 30 * time.Second
	max := 5 * time.Minute

	// Attempt 0 should return base
	if got := CalculateBackoff(base, max, 0); got != base {
		t.Errorf("attempt 0: got %s, want %s", got, base)
	}

	// Negative attempt should return base
	if got := CalculateBackoff(base, max, -1); got != base {
		t.Errorf("attempt -1: got %s, want %s", got, base)
	}

	// Very large attempt should be capped
	if got := CalculateBackoff(base, max, 100); got != max {
		t.Errorf("attempt 100: got %s, want %s", got, max)
	}
}
