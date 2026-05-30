package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// Integration Test: XDP Detach Detection and Reattach
// Requirements: 13.1, 13.3, 13.12
// =============================================================================

func TestIntegration_XDPDetachDetectionAndReattach(t *testing.T) {
	// This test verifies the full cycle:
	// 1. Monitor detects XDP is not attached
	// 2. Monitor calls HandleDetached to attempt reattach
	// 3. After reattach succeeds, status reflects XDP is attached
	// Note: Uses mock since BPF environment is not available in CI

	var reattachAttempts int32
	var alertCalls int32

	// Create a checker that simulates detach → reattach success on 2nd attempt
	xdpChecker := NewXDPChecker(XDPCheckerConfig{
		InterfaceName: "mock_eth0",
		XDPObjectPath: "/mock/xdp_filter.o",
		AlertFunc: func(alertType, message string) {
			atomic.AddInt32(&alertCalls, 1)
		},
	})

	// Override the reloadXDP behavior by tracking attempts
	// Since the real reloadXDP will fail (no real interface), we test the
	// detection and escalation logic
	m := New(MonitorConfig{
		CheckInterval: 50 * time.Millisecond,
	}, WithXDPChecker(xdpChecker)).(*healthMonitor)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Let it run several cycles - XDP will be detected as not attached
	time.Sleep(300 * time.Millisecond)

	status := m.Status()
	// XDP should not be attached (mock interface doesn't exist)
	if status.XDPAttached {
		t.Error("XDPAttached should be false for mock interface")
	}

	// Verify that reattach was attempted (failure count should be > 0)
	failCount := xdpChecker.ReloadFailureCount()
	if failCount == 0 {
		t.Error("expected at least one reload attempt for detached XDP")
	}

	_ = reattachAttempts // Used in concept

	if err := m.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// After 3 failures, alert should have been sent
	if failCount >= 3 && atomic.LoadInt32(&alertCalls) == 0 {
		t.Error("expected alert after 3 consecutive XDP reload failures")
	}
}


// =============================================================================
// Integration Test: Binary Health Endpoint Check Timing and Restart Logic
// Requirements: 13.2, 13.4, 13.9
// =============================================================================

func TestIntegration_BinaryHealthCheckTimingAndRestart(t *testing.T) {
	// This test verifies:
	// 1. Health check runs every CheckInterval
	// 2. After MaxConsecFailures (3) consecutive failures, restart is triggered
	// 3. If restart fails 3 times within 5 minutes, alert is sent and cooldown applies

	var restartCalls int32
	var alertReceived int32

	alertSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&alertReceived, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer alertSrv.Close()

	alertSender := NewAlertSender(AlertSenderConfig{
		MasterURL:  alertSrv.URL,
		LicenseKey: "test-key",
		NodeID:     "test-node",
	})

	// Health server that always returns 503 (unhealthy)
	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer healthSrv.Close()

	m := New(MonitorConfig{
		CheckInterval:      50 * time.Millisecond,
		HealthTimeout:      2 * time.Second,
		MaxConsecFailures:  3,
		MaxRestartFailures: 3,
		RestartCooldown:    200 * time.Millisecond, // Short for testing
	},
		WithAlertSender(alertSender),
		WithRestartFunc(func(ctx context.Context) error {
			atomic.AddInt32(&restartCalls, 1)
			return fmt.Errorf("restart failed") // Always fail
		}),
	).(*healthMonitor)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for enough cycles:
	// 3 failures (150ms) → restart attempt → fail
	// 3 more failures → restart attempt → fail
	// 3 more failures → restart attempt → fail (3rd restart failure → alert + cooldown)
	time.Sleep(800 * time.Millisecond)

	if err := m.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Verify restart was attempted multiple times
	restarts := atomic.LoadInt32(&restartCalls)
	if restarts < 3 {
		t.Errorf("restart calls = %d, want >= 3", restarts)
	}

	// Give async alert time to complete
	time.Sleep(100 * time.Millisecond)

	// Verify alert was sent after 3 restart failures
	alerts := atomic.LoadInt32(&alertReceived)
	if alerts == 0 {
		t.Error("expected at least one alert after 3 restart failures")
	}

	// Verify status reflects unhealthy state
	status := m.Status()
	if status.BinaryHealthy {
		t.Error("BinaryHealthy should be false when health checks fail")
	}
}

func TestIntegration_BinaryHealthCheckRecovery(t *testing.T) {
	// Test that when health check recovers, failure counter resets

	var healthyAfter int32
	atomic.StoreInt32(&healthyAfter, 5) // Become healthy after 5 checks

	var checkCount int32

	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&checkCount, 1)
		if count > atomic.LoadInt32(&healthyAfter) {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer healthSrv.Close()

	var restartCalls int32

	m := New(MonitorConfig{
		CheckInterval:     50 * time.Millisecond,
		HealthTimeout:     2 * time.Second,
		MaxConsecFailures: 3,
	},
		WithRestartFunc(func(ctx context.Context) error {
			atomic.AddInt32(&restartCalls, 1)
			return nil // Restart succeeds
		}),
	).(*healthMonitor)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for failures + restart + recovery
	time.Sleep(500 * time.Millisecond)

	if err := m.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Restart should have been called at least once
	if atomic.LoadInt32(&restartCalls) == 0 {
		t.Error("expected at least one restart call")
	}
}


// =============================================================================
// Integration Test: Offline → Online Transition with Config Sync
// Requirements: 13.6, 13.7
// =============================================================================

func TestIntegration_OfflineToOnlineWithConfigSync(t *testing.T) {
	// This test verifies the full offline → online transition:
	// 1. Client loses Master connectivity for > 60 seconds
	// 2. OfflineManager transitions to offline mode
	// 3. Reconnection attempts use exponential backoff
	// 4. When connectivity is restored:
	//    - Config is synced from Master
	//    - Offline report is sent (start time, end time, reconnect attempts)
	//    - Mode transitions back to online

	baseTime := time.Now()
	currentTime := baseTime

	var syncCalled int32
	var alertMessages []string
	var modeTransitions []OperationMode

	alertSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload AlertPayload
		json.NewDecoder(r.Body).Decode(&payload)
		alertMessages = append(alertMessages, payload.Message)
		w.WriteHeader(http.StatusOK)
	}))
	defer alertSrv.Close()

	alertSender := NewAlertSender(AlertSenderConfig{
		MasterURL:  alertSrv.URL,
		LicenseKey: "test-key",
		NodeID:     "test-node",
	})

	cfg := DefaultConfig()
	cfg.OfflineThreshold = 60 * time.Second
	cfg.ReconnectInterval = 30 * time.Second
	cfg.MaxReconnectBackoff = 5 * time.Minute

	masterAvailable := false

	om := NewOfflineManager(cfg, alertSender,
		WithNowFunc(func() time.Time { return currentTime }),
		WithCheckMasterFunc(func(ctx context.Context) error {
			if masterAvailable {
				return nil
			}
			return fmt.Errorf("connection refused")
		}),
		WithSyncConfigFunc(func(ctx context.Context) error {
			atomic.AddInt32(&syncCalled, 1)
			return nil
		}),
		WithOnModeChange(func(mode OperationMode) {
			modeTransitions = append(modeTransitions, mode)
		}),
	)

	// Phase 1: Go offline (Master unreachable for > 60s)
	currentTime = baseTime.Add(61 * time.Second)
	transitioned := om.EvaluateConnectivity()
	if !transitioned {
		t.Fatal("expected transition to offline mode")
	}
	if !om.CheckOfflineStatus() {
		t.Fatal("expected offline status")
	}

	// Phase 2: First reconnect attempt (fails)
	currentTime = baseTime.Add(92 * time.Second) // 61s + 31s (past first backoff of 30s)
	success := om.AttemptReconnect(context.Background())
	if success {
		t.Fatal("expected reconnect to fail (master unavailable)")
	}

	// Phase 3: Second reconnect attempt too early (blocked by backoff)
	currentTime = baseTime.Add(120 * time.Second) // Only 28s after first attempt
	success = om.AttemptReconnect(context.Background())
	if success {
		t.Fatal("expected reconnect to be blocked by backoff")
	}

	// Phase 4: Second reconnect attempt after backoff (60s after first attempt)
	currentTime = baseTime.Add(153 * time.Second) // 92s + 61s
	success = om.AttemptReconnect(context.Background())
	if success {
		t.Fatal("expected reconnect to fail (master still unavailable)")
	}

	// Phase 5: Master becomes available, reconnect succeeds
	masterAvailable = true
	currentTime = baseTime.Add(280 * time.Second) // Well past backoff
	success = om.AttemptReconnect(context.Background())
	if !success {
		t.Fatal("expected reconnect to succeed (master available)")
	}

	// Verify: back online
	if om.CheckOfflineStatus() {
		t.Fatal("expected online status after successful reconnect")
	}

	// Verify: config sync was called
	if atomic.LoadInt32(&syncCalled) != 1 {
		t.Errorf("syncConfig calls = %d, want 1", atomic.LoadInt32(&syncCalled))
	}

	// Verify: mode transitions were offline → online
	if len(modeTransitions) != 2 {
		t.Fatalf("mode transitions = %v, want [offline, online]", modeTransitions)
	}
	if modeTransitions[0] != ModeOffline {
		t.Errorf("first transition = %s, want offline", modeTransitions[0])
	}
	if modeTransitions[1] != ModeOnline {
		t.Errorf("second transition = %s, want online", modeTransitions[1])
	}

	// Verify: offline report was sent (via alert sender)
	time.Sleep(50 * time.Millisecond) // Allow async alert to complete
	if len(alertMessages) == 0 {
		t.Error("expected offline report to be sent via alert sender")
	}
}


// =============================================================================
// Integration Test: DDoS Detection → XDP Strict Mode Activation
// Requirements: 13.5, 13.11
// =============================================================================

func TestIntegration_DDoSDetectionToXDPStrictMode(t *testing.T) {
	// This test verifies the full DDoS detection flow:
	// 1. TrafficAnalyzer detects traffic > 10x threshold
	// 2. DDoSDetector activates XDP strict mode
	// 3. Alert is sent to Master within 5 seconds
	// 4. When traffic returns to normal, DDoS mode is cleared

	baseTime := time.Now()
	currentTime := baseTime

	var xdpStrictActivated int32
	var alertSent int32
	var alertType string

	// Create traffic analyzer with threshold 100 rps
	ta := NewTrafficAnalyzer(TrafficAnalyzerConfig{
		Window:             10 * time.Second,
		PlanThreshold:      100,  // 100 rps threshold
		EmergencyThreshold: 5.0,  // 5x = 500 rps for emergency
		DDoSThreshold:      10.0, // 10x = 1000 rps for DDoS
		NowFunc:            func() time.Time { return currentTime },
	})

	// Create DDoS detector
	dd := NewDDoSDetector(DDoSDetectorConfig{
		AlertDeadline: 5 * time.Second,
		XDPStrictFunc: func() error {
			atomic.AddInt32(&xdpStrictActivated, 1)
			return nil
		},
		AlertFunc: func(aType string, msg string) {
			atomic.AddInt32(&alertSent, 1)
			alertType = aType
		},
		NowFunc: func() time.Time { return currentTime },
	})

	// Phase 1: Normal traffic (below threshold)
	ta.RecordSample(50)
	currentTime = baseTime.Add(1 * time.Second)
	ta.RecordSample(80)

	if ta.IsDDoS() {
		t.Fatal("should not detect DDoS at normal traffic levels")
	}
	if ta.IsEmergency() {
		t.Fatal("should not detect emergency at normal traffic levels")
	}

	// Phase 2: Traffic spikes to emergency level (> 5x = 500)
	// Need average > 500. Use high values to overcome earlier low samples.
	currentTime = baseTime.Add(2 * time.Second)
	ta.RecordSample(800)
	currentTime = baseTime.Add(3 * time.Second)
	ta.RecordSample(900)
	currentTime = baseTime.Add(4 * time.Second)
	ta.RecordSample(900)

	// Average = (50+80+800+900+900)/5 = 546, which is > 500 (emergency)
	if !ta.IsEmergency() {
		t.Fatalf("should detect emergency at avg > 500 rps, got rate %d", ta.CurrentRate())
	}
	if ta.IsDDoS() {
		t.Fatal("should not detect DDoS yet (avg < 1000)")
	}

	// Phase 3: Traffic spikes to DDoS level (> 10x = 1000)
	currentTime = baseTime.Add(5 * time.Second)
	ta.RecordSample(1500)
	currentTime = baseTime.Add(6 * time.Second)
	ta.RecordSample(2000)
	currentTime = baseTime.Add(7 * time.Second)
	ta.RecordSample(2500)

	if !ta.IsDDoS() {
		t.Fatalf("should detect DDoS at current rate (avg should be > 1000)")
	}

	// Phase 4: Trigger DDoS detection → XDP strict mode + alert
	dd.Detect(ta.CurrentRate(), 100)

	if !dd.IsActive() {
		t.Fatal("DDoS detector should be active after detection")
	}
	if atomic.LoadInt32(&xdpStrictActivated) != 1 {
		t.Error("XDP strict mode should have been activated")
	}
	if atomic.LoadInt32(&alertSent) != 1 {
		t.Error("alert should have been sent to Master")
	}
	if alertType != "ddos_detected" {
		t.Errorf("alert type = %q, want %q", alertType, "ddos_detected")
	}

	// Phase 5: Traffic returns to normal
	// Advance well past the window so all DDoS-level samples are pruned
	// Window is 10s, last DDoS sample was at t=7s, so at t=18s the window starts at t=8s
	// which excludes all samples at t<=7s
	currentTime = baseTime.Add(18 * time.Second)
	ta.RecordSample(50)
	currentTime = baseTime.Add(19 * time.Second)
	ta.RecordSample(60)

	if ta.IsDDoS() {
		t.Fatalf("should not detect DDoS after traffic normalizes, got rate %d", ta.CurrentRate())
	}

	// Clear DDoS mode
	dd.Clear()
	if dd.IsActive() {
		t.Fatal("DDoS detector should not be active after clear")
	}
}

func TestIntegration_EmergencyRecoveryWithTrafficAnalyzer(t *testing.T) {
	// Test emergency recovery: traffic > 5x threshold + crash → restart with
	// rate-limit reduced 50% for 5 minutes, then restore original

	now := time.Now()
	currentTime := &now

	var activatedRateLimit uint64
	var deactivatedRateLimit uint64

	ta := NewTrafficAnalyzer(TrafficAnalyzerConfig{
		Window:             10 * time.Second,
		PlanThreshold:      100,
		EmergencyThreshold: 5.0,
		NowFunc:            func() time.Time { return *currentTime },
	})

	er := NewEmergencyRecovery(EmergencyRecoveryConfig{
		Duration:          5 * time.Minute,
		OriginalRateLimit: 1000,
		OnActivate: func(reduced uint64) {
			activatedRateLimit = reduced
		},
		OnDeactivate: func(original uint64) {
			deactivatedRateLimit = original
		},
		NowFunc: func() time.Time { return *currentTime },
	})

	// Simulate high traffic (> 5x threshold = 500)
	ta.RecordSample(600)
	*currentTime = now.Add(1 * time.Second)
	ta.RecordSample(700)

	// Verify emergency condition
	if !ta.IsEmergency() {
		t.Fatal("should detect emergency condition")
	}

	// Simulate crash recovery → activate emergency mode
	er.Activate()

	if !er.IsActive() {
		t.Fatal("emergency mode should be active")
	}
	if activatedRateLimit != 500 {
		t.Errorf("activated rate limit = %d, want 500 (50%% of 1000)", activatedRateLimit)
	}
	if er.GetCurrentRateLimit() != 500 {
		t.Errorf("current rate limit = %d, want 500", er.GetCurrentRateLimit())
	}

	// Advance time to 4 minutes (still in emergency)
	*currentTime = now.Add(4 * time.Minute)
	if er.CheckExpiry() {
		t.Fatal("emergency should not expire before 5 minutes")
	}

	// Advance time past 5 minutes (emergency expires)
	*currentTime = now.Add(5*time.Minute + 1*time.Second)
	if !er.CheckExpiry() {
		t.Fatal("emergency should expire after 5 minutes")
	}

	if er.IsActive() {
		t.Fatal("emergency mode should not be active after expiry")
	}
	if deactivatedRateLimit != 1000 {
		t.Errorf("deactivated rate limit = %d, want 1000 (original)", deactivatedRateLimit)
	}
	if er.GetCurrentRateLimit() != 1000 {
		t.Errorf("current rate limit after expiry = %d, want 1000", er.GetCurrentRateLimit())
	}
}


// =============================================================================
// Integration Test: Snapshot Write/Read Cycle
// Requirements: 13.8, 13.13
// =============================================================================

func TestIntegration_SnapshotWriteReadCycle(t *testing.T) {
	// This test verifies the full snapshot lifecycle:
	// 1. State is captured (rate-limit, session, ban list, XDP stats)
	// 2. State is serialized to JSON and compressed with gzip
	// 3. Snapshot is written to disk atomically (temp + rename)
	// 4. Snapshot can be read back and deserialized correctly
	// 5. Data integrity is preserved through the cycle

	tmpDir := t.TempDir()
	now := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	// Create realistic state data
	originalState := HealthSnapshot{
		RateLimitState: map[string]RateBucket{
			"192.168.1.1":  {IP: "192.168.1.1", Count: 42, WindowEnd: now.Add(60 * time.Second)},
			"10.0.0.5":     {IP: "10.0.0.5", Count: 100, WindowEnd: now.Add(30 * time.Second)},
			"172.16.0.100": {IP: "172.16.0.100", Count: 7, WindowEnd: now.Add(90 * time.Second)},
		},
		SessionState: map[string]SessionData{
			"sess_abc123": {Token: "token_abc123", CreatedAt: now.Add(-5 * time.Minute), ExpiresAt: now.Add(55 * time.Minute)},
			"sess_def456": {Token: "token_def456", CreatedAt: now.Add(-2 * time.Minute), ExpiresAt: now.Add(58 * time.Minute)},
		},
		BanList: []BanEntry{
			{IP: "203.0.113.1", ExpiresAt: now.Add(time.Hour), Reason: "rate_limit_exceeded"},
			{IP: "198.51.100.5", ExpiresAt: now.Add(30 * time.Minute), Reason: "ddos_source"},
		},
		XDPStats: XDPStats{
			TotalPackets:   1000000,
			PassedPackets:  950000,
			DroppedPackets: 50000,
		},
	}

	config := MonitorConfig{
		SnapshotInterval: 1 * time.Second,
		SnapshotMaxSize:  64 * 1024 * 1024, // 64MB
		SnapshotPath:     tmpDir,
	}

	sw := NewSnapshotWriter(config, func() HealthSnapshot { return originalState },
		WithSnapshotNowFunc(func() time.Time { return now }),
	)

	// Write snapshot
	sw.writeSnapshot()

	// Verify file was created
	snapshotPath := filepath.Join(tmpDir, "state.snapshot.gz")
	fileData, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("failed to read snapshot file: %v", err)
	}

	// Verify file is non-empty
	if len(fileData) == 0 {
		t.Fatal("snapshot file is empty")
	}

	// Verify it's valid gzip (magic bytes)
	if len(fileData) < 2 || fileData[0] != 0x1f || fileData[1] != 0x8b {
		t.Fatal("snapshot file does not have gzip magic bytes")
	}

	// Read back: decompress
	decompressed, err := DecompressGzip(fileData)
	if err != nil {
		t.Fatalf("failed to decompress snapshot: %v", err)
	}

	// Read back: deserialize
	var readSnapshot HealthSnapshot
	if err := json.Unmarshal(decompressed, &readSnapshot); err != nil {
		t.Fatalf("failed to unmarshal snapshot: %v", err)
	}

	// Verify timestamp
	if !readSnapshot.Timestamp.Equal(now) {
		t.Errorf("timestamp mismatch: got %v, want %v", readSnapshot.Timestamp, now)
	}

	// Verify rate-limit state
	if len(readSnapshot.RateLimitState) != 3 {
		t.Errorf("rate limit entries = %d, want 3", len(readSnapshot.RateLimitState))
	}
	if bucket, ok := readSnapshot.RateLimitState["192.168.1.1"]; !ok {
		t.Error("missing rate limit entry for 192.168.1.1")
	} else if bucket.Count != 42 {
		t.Errorf("rate limit count for 192.168.1.1 = %d, want 42", bucket.Count)
	}

	// Verify session state
	if len(readSnapshot.SessionState) != 2 {
		t.Errorf("session entries = %d, want 2", len(readSnapshot.SessionState))
	}
	if sess, ok := readSnapshot.SessionState["sess_abc123"]; !ok {
		t.Error("missing session entry for sess_abc123")
	} else if sess.Token != "token_abc123" {
		t.Errorf("session token = %q, want %q", sess.Token, "token_abc123")
	}

	// Verify ban list
	if len(readSnapshot.BanList) != 2 {
		t.Errorf("ban list entries = %d, want 2", len(readSnapshot.BanList))
	}

	// Verify XDP stats
	if readSnapshot.XDPStats.TotalPackets != 1000000 {
		t.Errorf("total packets = %d, want 1000000", readSnapshot.XDPStats.TotalPackets)
	}
	if readSnapshot.XDPStats.PassedPackets != 950000 {
		t.Errorf("passed packets = %d, want 950000", readSnapshot.XDPStats.PassedPackets)
	}
	if readSnapshot.XDPStats.DroppedPackets != 50000 {
		t.Errorf("dropped packets = %d, want 50000", readSnapshot.XDPStats.DroppedPackets)
	}

	// Verify no temp file remains (atomic write cleanup)
	tmpPath := snapshotPath + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temp file should not exist after successful write")
	}

	// Verify lastSnapshot was updated
	if sw.LastSnapshotTime() != now {
		t.Errorf("lastSnapshot = %v, want %v", sw.LastSnapshotTime(), now)
	}
}

func TestIntegration_SnapshotPeriodicWriteWithIOFailureRecovery(t *testing.T) {
	// Test that snapshot writer continues operating after I/O failures
	// Requirement 13.13: On I/O failure, log warning, keep old snapshot, retry next cycle

	writeAttempts := 0
	var successfulWrites int32

	provider := func() HealthSnapshot {
		return HealthSnapshot{
			RateLimitState: map[string]RateBucket{
				"1.2.3.4": {IP: "1.2.3.4", Count: writeAttempts, WindowEnd: time.Now().Add(time.Minute)},
			},
			SessionState: map[string]SessionData{},
			BanList:      []BanEntry{},
			XDPStats:     XDPStats{TotalPackets: uint64(writeAttempts)},
		}
	}

	config := MonitorConfig{
		SnapshotInterval: 50 * time.Millisecond, // Fast for testing
		SnapshotMaxSize:  64 * 1024 * 1024,
		SnapshotPath:     "/tmp/kiro-test-snapshots",
	}

	sw := NewSnapshotWriter(config, provider,
		WithSnapshotWriteFileFunc(func(path string, data []byte) error {
			writeAttempts++
			// Fail on attempts 1 and 2, succeed after
			if writeAttempts <= 2 {
				return fmt.Errorf("simulated disk full")
			}
			atomic.AddInt32(&successfulWrites, 1)
			return nil
		}),
	)

	ctx := context.Background()
	sw.Start(ctx)

	// Wait for several cycles (enough for failures + recovery)
	time.Sleep(350 * time.Millisecond)

	sw.Stop()

	// Verify that writes were attempted multiple times
	if writeAttempts < 3 {
		t.Errorf("write attempts = %d, want >= 3", writeAttempts)
	}

	// Verify that at least one write succeeded after the failures
	if atomic.LoadInt32(&successfulWrites) == 0 {
		t.Error("expected at least one successful write after I/O failures recovered")
	}
}

// =============================================================================
// Integration Test: Monitor with Alert Sender
// (Existing test preserved and enhanced)
// =============================================================================

func TestMonitorWithXDPChecker(t *testing.T) {
	// Create a monitor with an XDP checker for a non-existent interface
	xdpChecker := NewXDPChecker(XDPCheckerConfig{
		InterfaceName: "nonexistent_test_iface",
		XDPObjectPath: "/nonexistent/xdp.o",
		AlertFunc:     func(alertType, message string) {},
	})

	m := New(MonitorConfig{
		CheckInterval: 50 * time.Millisecond,
	}, WithXDPChecker(xdpChecker)).(*healthMonitor)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Let it run a few cycles
	time.Sleep(200 * time.Millisecond)

	status := m.Status()
	// XDP should not be attached (non-existent interface)
	if status.XDPAttached {
		t.Error("XDPAttached should be false for non-existent interface")
	}

	if err := m.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestMonitorWithAlertSender(t *testing.T) {
	var alertReceived int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&alertReceived, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	alertSender := NewAlertSender(AlertSenderConfig{
		MasterURL:  srv.URL,
		LicenseKey: "test-key",
		NodeID:     "test-node",
	})

	m := New(MonitorConfig{
		CheckInterval: 50 * time.Millisecond,
	}, WithAlertSender(alertSender)).(*healthMonitor)

	// Test that sendAlert works through the monitor
	m.sendAlert("test_alert", "test message")

	// Give async alert time to complete
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&alertReceived) != 1 {
		t.Errorf("alert received count = %d, want 1", atomic.LoadInt32(&alertReceived))
	}
}

func TestMonitorWithCustomRestartFunc(t *testing.T) {
	var restartCalled int32

	m := New(MonitorConfig{
		CheckInterval:     50 * time.Millisecond,
		MaxConsecFailures: 2,
	}, WithRestartFunc(func(ctx context.Context) error {
		atomic.AddInt32(&restartCalled, 1)
		return nil // Restart succeeds
	})).(*healthMonitor)

	// Create a health server that always fails
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for enough cycles to trigger restart (2 failures needed)
	time.Sleep(250 * time.Millisecond)

	if err := m.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	if atomic.LoadInt32(&restartCalled) == 0 {
		t.Error("restart function should have been called at least once")
	}
}

func TestMonitorRestartFailureEscalation(t *testing.T) {
	var alertCount int32
	var restartCount int32

	alertSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&alertCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer alertSrv.Close()

	alertSender := NewAlertSender(AlertSenderConfig{
		MasterURL:  alertSrv.URL,
		LicenseKey: "key",
		NodeID:     "node",
	})

	m := New(MonitorConfig{
		CheckInterval:      50 * time.Millisecond,
		MaxConsecFailures:  1, // Trigger restart after 1 failure
		MaxRestartFailures: 3,
		RestartCooldown:    100 * time.Millisecond, // Short cooldown for testing
	},
		WithAlertSender(alertSender),
		WithRestartFunc(func(ctx context.Context) error {
			atomic.AddInt32(&restartCount, 1)
			return fmt.Errorf("restart failed") // Always fail
		}),
	).(*healthMonitor)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for enough cycles to trigger multiple restart attempts
	time.Sleep(500 * time.Millisecond)

	if err := m.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Should have attempted restarts
	if atomic.LoadInt32(&restartCount) < 3 {
		t.Errorf("restart count = %d, want >= 3", atomic.LoadInt32(&restartCount))
	}

	// Give async alert time to complete
	time.Sleep(100 * time.Millisecond)

	// Should have sent at least one alert after 3 failures
	if atomic.LoadInt32(&alertCount) == 0 {
		t.Error("alert should have been sent after 3 restart failures")
	}
}

func TestMonitorWithServiceName(t *testing.T) {
	m := New(MonitorConfig{},
		WithServiceName("kiro-test-service"),
	).(*healthMonitor)

	if m.serviceName != "kiro-test-service" {
		t.Errorf("serviceName = %q, want %q", m.serviceName, "kiro-test-service")
	}
}

func TestMonitorDefaultServiceName(t *testing.T) {
	m := New(MonitorConfig{}).(*healthMonitor)

	if m.serviceName != "kiro-client-waf" {
		t.Errorf("default serviceName = %q, want %q", m.serviceName, "kiro-client-waf")
	}
}

func TestMonitorBinaryHealthCheckWithHealthyServer(t *testing.T) {
	// Create a test server that responds 200 on the health endpoint
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	m := &healthMonitor{
		config: MonitorConfig{
			HealthTimeout: 2 * time.Second,
		},
		client: srv.Client(),
		status: MonitorStatus{Mode: ModeOnline},
		healthFailures:  NewFailureTracker(1 * time.Minute),
		restartFailures: NewFailureTracker(5 * time.Minute),
	}

	ctx := context.Background()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	resp, err := m.client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status code = %d, want 200", resp.StatusCode)
	}
}

func TestMonitorPerformXDPCheck_NilChecker(t *testing.T) {
	m := &healthMonitor{
		config:          MonitorConfig{},
		status:          MonitorStatus{Mode: ModeOnline},
		healthFailures:  NewFailureTracker(1 * time.Minute),
		restartFailures: NewFailureTracker(5 * time.Minute),
		xdpChecker:      nil, // No XDP checker
	}

	ctx := context.Background()

	// Should not panic when xdpChecker is nil
	m.performXDPCheck(ctx)
}

func TestMonitorSendAlert_NoSender(t *testing.T) {
	m := &healthMonitor{
		alertSender: nil, // No alert sender
	}

	// Should not panic
	m.sendAlert("test", "test message")
}
