package escalation

import (
	"testing"
	"time"
)

func TestNewEscalationEngine(t *testing.T) {
	config := EscalationConfig{
		FailureThreshold: 3,
		FailureWindow:    5 * time.Minute,
		CooldownDuration: 10 * time.Minute,
	}
	adminIPs := []string{"10.0.0.1", "192.168.1.1"}

	engine := NewEscalationEngine(config, adminIPs)

	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
	if len(engine.adminAllowlist) != 2 {
		t.Fatalf("expected 2 admin IPs, got %d", len(engine.adminAllowlist))
	}
	if !engine.adminAllowlist["10.0.0.1"] {
		t.Error("expected 10.0.0.1 in admin allowlist")
	}
	if !engine.adminAllowlist["192.168.1.1"] {
		t.Error("expected 192.168.1.1 in admin allowlist")
	}
}

func TestGetLevel_AdminIPReturnsZero(t *testing.T) {
	config := EscalationConfig{
		FailureThreshold: 3,
		FailureWindow:    5 * time.Minute,
		CooldownDuration: 10 * time.Minute,
	}
	engine := NewEscalationEngine(config, []string{"10.0.0.1"})

	level := engine.GetLevel("10.0.0.1")
	if level != 0 {
		t.Errorf("expected level 0 for admin IP, got %d", level)
	}
}

func TestGetLevel_NewVisitorReturnsOne(t *testing.T) {
	config := EscalationConfig{
		FailureThreshold: 3,
		FailureWindow:    5 * time.Minute,
		CooldownDuration: 10 * time.Minute,
	}
	engine := NewEscalationEngine(config, nil)

	level := engine.GetLevel("1.2.3.4")
	if level != 1 {
		t.Errorf("expected level 1 for new visitor, got %d", level)
	}
}

func TestRecordFailure_EscalatesAfterThreshold(t *testing.T) {
	config := EscalationConfig{
		FailureThreshold: 3,
		FailureWindow:    5 * time.Minute,
		CooldownDuration: 10 * time.Minute,
	}
	engine := NewEscalationEngine(config, nil)

	ip := "1.2.3.4"

	// Record failures up to threshold (3 failures don't escalate, 4th does)
	engine.RecordFailure(ip, "transparent")
	engine.RecordFailure(ip, "transparent")
	engine.RecordFailure(ip, "transparent")

	// At threshold, not yet escalated
	engine.mu.RLock()
	state := engine.states[ip]
	levelBeforeEscalation := state.Level
	engine.mu.RUnlock()

	if levelBeforeEscalation != 1 {
		t.Errorf("expected level 1 before exceeding threshold, got %d", levelBeforeEscalation)
	}

	// One more failure exceeds threshold → escalate
	engine.RecordFailure(ip, "transparent")

	engine.mu.RLock()
	state = engine.states[ip]
	levelAfterEscalation := state.Level
	engine.mu.RUnlock()

	if levelAfterEscalation != 2 {
		t.Errorf("expected level 2 after exceeding threshold, got %d", levelAfterEscalation)
	}
}

func TestRecordFailure_MaxLevelIsFour(t *testing.T) {
	config := EscalationConfig{
		FailureThreshold: 1,
		FailureWindow:    5 * time.Minute,
		CooldownDuration: 10 * time.Minute,
	}
	engine := NewEscalationEngine(config, nil)

	ip := "1.2.3.4"

	// Escalate multiple times (threshold=1, so 2 failures per escalation)
	for i := 0; i < 20; i++ {
		engine.RecordFailure(ip, "any")
	}

	engine.mu.RLock()
	state := engine.states[ip]
	level := state.Level
	engine.mu.RUnlock()

	if level != 4 {
		t.Errorf("expected max level 4, got %d", level)
	}
}

func TestRecordSuccess_ResetsFailureCount(t *testing.T) {
	config := EscalationConfig{
		FailureThreshold: 3,
		FailureWindow:    5 * time.Minute,
		CooldownDuration: 10 * time.Minute,
	}
	engine := NewEscalationEngine(config, nil)

	ip := "1.2.3.4"

	// Record some failures
	engine.RecordFailure(ip, "transparent")
	engine.RecordFailure(ip, "transparent")

	// Record success
	engine.RecordSuccess(ip)

	engine.mu.RLock()
	state := engine.states[ip]
	failureCount := state.FailureCount
	level := state.Level
	engine.mu.RUnlock()

	if failureCount != 0 {
		t.Errorf("expected failure count 0 after success, got %d", failureCount)
	}
	// Level should NOT de-escalate on success
	if level != 1 {
		t.Errorf("expected level 1 (no de-escalation on success), got %d", level)
	}
}

func TestGetLevel_DeEscalatesAfterCooldown(t *testing.T) {
	config := EscalationConfig{
		FailureThreshold: 1,
		FailureWindow:    5 * time.Minute,
		CooldownDuration: 10 * time.Minute,
	}
	engine := NewEscalationEngine(config, nil)

	now := time.Now()
	engine.SetNowFunc(func() time.Time { return now })

	ip := "1.2.3.4"

	// Escalate to level 2
	engine.RecordFailure(ip, "transparent")
	engine.RecordFailure(ip, "transparent")

	engine.mu.RLock()
	level := engine.states[ip].Level
	engine.mu.RUnlock()
	if level != 2 {
		t.Fatalf("expected level 2, got %d", level)
	}

	// Advance time past cooldown
	now = now.Add(11 * time.Minute)
	engine.SetNowFunc(func() time.Time { return now })

	// GetLevel should de-escalate
	gotLevel := engine.GetLevel(ip)
	if gotLevel != 1 {
		t.Errorf("expected level 1 after cooldown, got %d", gotLevel)
	}
}

func TestGetLevel_DeEscalatesToZero(t *testing.T) {
	config := EscalationConfig{
		FailureThreshold: 1,
		FailureWindow:    5 * time.Minute,
		CooldownDuration: 10 * time.Minute,
	}
	engine := NewEscalationEngine(config, nil)

	now := time.Now()
	engine.SetNowFunc(func() time.Time { return now })

	ip := "1.2.3.4"

	// Escalate to level 2
	engine.RecordFailure(ip, "transparent")
	engine.RecordFailure(ip, "transparent")

	// Advance time past 2× cooldown (should de-escalate by 2 levels: 2→0)
	now = now.Add(21 * time.Minute)
	engine.SetNowFunc(func() time.Time { return now })

	gotLevel := engine.GetLevel(ip)
	if gotLevel != 0 {
		t.Errorf("expected level 0 after 2× cooldown, got %d", gotLevel)
	}
}

func TestRecordFailure_ResetsCountOutsideWindow(t *testing.T) {
	config := EscalationConfig{
		FailureThreshold: 3,
		FailureWindow:    5 * time.Minute,
		CooldownDuration: 10 * time.Minute,
	}
	engine := NewEscalationEngine(config, nil)

	now := time.Now()
	engine.SetNowFunc(func() time.Time { return now })

	ip := "1.2.3.4"

	// Record 2 failures
	engine.RecordFailure(ip, "transparent")
	engine.RecordFailure(ip, "transparent")

	// Advance time past failure window
	now = now.Add(6 * time.Minute)
	engine.SetNowFunc(func() time.Time { return now })

	// Record another failure — count should reset first
	engine.RecordFailure(ip, "transparent")

	engine.mu.RLock()
	state := engine.states[ip]
	failureCount := state.FailureCount
	engine.mu.RUnlock()

	if failureCount != 1 {
		t.Errorf("expected failure count 1 after window reset, got %d", failureCount)
	}
}

func TestCleanup_RemovesStaleEntries(t *testing.T) {
	config := EscalationConfig{
		FailureThreshold: 3,
		FailureWindow:    5 * time.Minute,
		CooldownDuration: 10 * time.Minute,
	}
	engine := NewEscalationEngine(config, nil)

	now := time.Now()
	engine.SetNowFunc(func() time.Time { return now })

	// Create entries
	engine.RecordFailure("1.1.1.1", "transparent")
	engine.RecordFailure("2.2.2.2", "transparent")

	// Advance time past 2× cooldown (stale threshold)
	now = now.Add(21 * time.Minute)
	engine.SetNowFunc(func() time.Time { return now })

	// Record activity for one IP to keep it fresh
	engine.RecordFailure("2.2.2.2", "transparent")

	// Cleanup should remove stale entry
	engine.Cleanup()

	engine.mu.RLock()
	_, exists1 := engine.states["1.1.1.1"]
	_, exists2 := engine.states["2.2.2.2"]
	engine.mu.RUnlock()

	if exists1 {
		t.Error("expected 1.1.1.1 to be cleaned up (stale)")
	}
	if !exists2 {
		t.Error("expected 2.2.2.2 to still exist (recent activity)")
	}
}

func TestRecordSuccess_NoOpForUnknownIP(t *testing.T) {
	config := EscalationConfig{
		FailureThreshold: 3,
		FailureWindow:    5 * time.Minute,
		CooldownDuration: 10 * time.Minute,
	}
	engine := NewEscalationEngine(config, nil)

	// Should not panic or create state for unknown IP
	engine.RecordSuccess("unknown.ip")

	engine.mu.RLock()
	_, exists := engine.states["unknown.ip"]
	engine.mu.RUnlock()

	if exists {
		t.Error("expected no state created for unknown IP on success")
	}
}
