package monitor

import (
	"testing"
	"time"
)

func TestTrafficAnalyzer_CurrentRate(t *testing.T) {
	baseTime := time.Now()
	currentTime := baseTime

	ta := NewTrafficAnalyzer(TrafficAnalyzerConfig{
		Window:        10 * time.Second,
		PlanThreshold: 100,
		NowFunc:       func() time.Time { return currentTime },
	})

	// No samples → rate is 0
	if rate := ta.CurrentRate(); rate != 0 {
		t.Fatalf("expected rate 0 with no samples, got %d", rate)
	}

	// Add samples
	ta.RecordSample(50)
	currentTime = baseTime.Add(1 * time.Second)
	ta.RecordSample(100)
	currentTime = baseTime.Add(2 * time.Second)
	ta.RecordSample(150)

	// Average should be (50 + 100 + 150) / 3 = 100
	rate := ta.CurrentRate()
	if rate != 100 {
		t.Fatalf("expected rate 100, got %d", rate)
	}
}

func TestTrafficAnalyzer_SlidingWindowPrune(t *testing.T) {
	baseTime := time.Now()
	currentTime := baseTime

	ta := NewTrafficAnalyzer(TrafficAnalyzerConfig{
		Window:        10 * time.Second,
		PlanThreshold: 100,
		NowFunc:       func() time.Time { return currentTime },
	})

	// Add sample at t=0
	ta.RecordSample(50)

	// Add sample at t=5s
	currentTime = baseTime.Add(5 * time.Second)
	ta.RecordSample(200)

	// At t=5s, both samples are in window: avg = (50+200)/2 = 125
	rate := ta.CurrentRate()
	if rate != 125 {
		t.Fatalf("expected rate 125 at t=5s, got %d", rate)
	}

	// Advance past window for first sample (t=11s, window is 10s)
	currentTime = baseTime.Add(11 * time.Second)
	ta.RecordSample(300)

	// Now only samples at t=5s and t=11s should be in window
	// windowStart = t=11s - 10s = t=1s
	// sample at t=5s (>= t=1s) → kept
	// sample at t=11s (>= t=1s) → kept
	// avg = (200 + 300) / 2 = 250
	rate = ta.CurrentRate()
	if rate != 250 {
		t.Fatalf("expected rate 250 at t=11s, got %d", rate)
	}
}

func TestTrafficAnalyzer_IsEmergency(t *testing.T) {
	baseTime := time.Now()
	currentTime := baseTime

	ta := NewTrafficAnalyzer(TrafficAnalyzerConfig{
		Window:             10 * time.Second,
		PlanThreshold:      100, // 100 rps threshold
		EmergencyThreshold: 5.0, // 5x = 500 rps
		NowFunc:            func() time.Time { return currentTime },
	})

	// Below threshold
	ta.RecordSample(400)
	if ta.IsEmergency() {
		t.Fatal("should not be emergency at 400 rps (threshold 500)")
	}

	// At threshold (not exceeding)
	currentTime = baseTime.Add(1 * time.Second)
	ta.RecordSample(500)
	// avg = (400+500)/2 = 450, still below 500
	if ta.IsEmergency() {
		t.Fatal("should not be emergency at avg 450 rps")
	}

	// Above threshold
	currentTime = baseTime.Add(2 * time.Second)
	ta.RecordSample(600)
	// avg = (400+500+600)/3 = 500, not exceeding (must be > 500)
	if ta.IsEmergency() {
		t.Fatal("should not be emergency at avg 500 rps (must exceed)")
	}

	currentTime = baseTime.Add(3 * time.Second)
	ta.RecordSample(700)
	// avg = (400+500+600+700)/4 = 550, exceeds 500
	if !ta.IsEmergency() {
		t.Fatal("should be emergency at avg 550 rps (threshold 500)")
	}
}

func TestTrafficAnalyzer_IsDDoS(t *testing.T) {
	baseTime := time.Now()
	currentTime := baseTime

	ta := NewTrafficAnalyzer(TrafficAnalyzerConfig{
		Window:        10 * time.Second,
		PlanThreshold: 100,  // 100 rps threshold
		DDoSThreshold: 10.0, // 10x = 1000 rps
		NowFunc:       func() time.Time { return currentTime },
	})

	// Below DDoS threshold
	ta.RecordSample(900)
	if ta.IsDDoS() {
		t.Fatal("should not be DDoS at 900 rps (threshold 1000)")
	}

	// At DDoS threshold (not exceeding)
	currentTime = baseTime.Add(1 * time.Second)
	ta.RecordSample(1000)
	// avg = (900+1000)/2 = 950, below 1000
	if ta.IsDDoS() {
		t.Fatal("should not be DDoS at avg 950 rps")
	}

	// Above DDoS threshold
	currentTime = baseTime.Add(2 * time.Second)
	ta.RecordSample(1200)
	// avg = (900+1000+1200)/3 = 1033, exceeds 1000
	if !ta.IsDDoS() {
		t.Fatal("should be DDoS at avg 1033 rps (threshold 1000)")
	}
}

func TestTrafficAnalyzer_ZeroPlanThreshold(t *testing.T) {
	ta := NewTrafficAnalyzer(TrafficAnalyzerConfig{
		Window:        10 * time.Second,
		PlanThreshold: 0, // No threshold set
	})

	ta.RecordSample(99999)

	// With zero threshold, should never trigger
	if ta.IsEmergency() {
		t.Fatal("should not be emergency with zero threshold")
	}
	if ta.IsDDoS() {
		t.Fatal("should not be DDoS with zero threshold")
	}
}

func TestEmergencyRecovery_ActivateAndExpire(t *testing.T) {
	now := time.Now()
	currentTime := &now

	var activatedWith uint64
	var deactivatedWith uint64

	er := NewEmergencyRecovery(EmergencyRecoveryConfig{
		Duration:          5 * time.Minute,
		OriginalRateLimit: 1000,
		OnActivate: func(reduced uint64) {
			activatedWith = reduced
		},
		OnDeactivate: func(original uint64) {
			deactivatedWith = original
		},
		NowFunc: func() time.Time { return *currentTime },
	})

	// Initially not active
	if er.IsActive() {
		t.Fatal("should not be active initially")
	}
	if er.GetCurrentRateLimit() != 1000 {
		t.Fatalf("expected original rate limit 1000, got %d", er.GetCurrentRateLimit())
	}

	// Activate emergency
	er.Activate()
	if !er.IsActive() {
		t.Fatal("should be active after activation")
	}
	if activatedWith != 500 {
		t.Fatalf("expected activation with 500 (50%% of 1000), got %d", activatedWith)
	}
	if er.GetCurrentRateLimit() != 500 {
		t.Fatalf("expected emergency rate limit 500, got %d", er.GetCurrentRateLimit())
	}

	// Double activation should be no-op
	er.Activate()

	// Check expiry before duration
	*currentTime = now.Add(4 * time.Minute)
	expired := er.CheckExpiry()
	if expired {
		t.Fatal("should not expire before 5 minutes")
	}
	if !er.IsActive() {
		t.Fatal("should still be active before 5 minutes")
	}

	// Check expiry after duration
	*currentTime = now.Add(5*time.Minute + 1*time.Second)
	expired = er.CheckExpiry()
	if !expired {
		t.Fatal("should expire after 5 minutes")
	}
	if er.IsActive() {
		t.Fatal("should not be active after expiry")
	}
	if deactivatedWith != 1000 {
		t.Fatalf("expected deactivation with original 1000, got %d", deactivatedWith)
	}
	if er.GetCurrentRateLimit() != 1000 {
		t.Fatalf("expected restored rate limit 1000, got %d", er.GetCurrentRateLimit())
	}
}

func TestEmergencyRecovery_RemainingDuration(t *testing.T) {
	now := time.Now()
	currentTime := &now

	er := NewEmergencyRecovery(EmergencyRecoveryConfig{
		Duration:          5 * time.Minute,
		OriginalRateLimit: 1000,
		NowFunc:           func() time.Time { return *currentTime },
	})

	// Not active → 0 remaining
	if er.RemainingDuration() != 0 {
		t.Fatal("expected 0 remaining when not active")
	}

	er.Activate()

	// Just activated → ~5 minutes remaining
	remaining := er.RemainingDuration()
	if remaining != 5*time.Minute {
		t.Fatalf("expected 5m remaining, got %s", remaining)
	}

	// After 2 minutes → ~3 minutes remaining
	*currentTime = now.Add(2 * time.Minute)
	remaining = er.RemainingDuration()
	if remaining != 3*time.Minute {
		t.Fatalf("expected 3m remaining, got %s", remaining)
	}
}

func TestDDoSDetector_DetectAndClear(t *testing.T) {
	var xdpStrictCalled bool
	var alertType, alertMessage string

	dd := NewDDoSDetector(DDoSDetectorConfig{
		AlertDeadline: 5 * time.Second,
		XDPStrictFunc: func() error {
			xdpStrictCalled = true
			return nil
		},
		AlertFunc: func(aType string, msg string) {
			alertType = aType
			alertMessage = msg
		},
	})

	// Initially not active
	if dd.IsActive() {
		t.Fatal("should not be active initially")
	}

	// Detect DDoS
	dd.Detect(1500, 100)

	if !dd.IsActive() {
		t.Fatal("should be active after detection")
	}
	if !xdpStrictCalled {
		t.Fatal("XDP strict mode should have been activated")
	}
	if alertType != "ddos_detected" {
		t.Fatalf("expected alert type 'ddos_detected', got '%s'", alertType)
	}
	if alertMessage == "" {
		t.Fatal("expected non-empty alert message")
	}

	// Double detection should be no-op
	xdpStrictCalled = false
	dd.Detect(2000, 100)
	if xdpStrictCalled {
		t.Fatal("should not call XDP strict again when already active")
	}

	// Clear
	dd.Clear()
	if dd.IsActive() {
		t.Fatal("should not be active after clear")
	}

	// Clear when not active should be no-op
	dd.Clear()
}

func TestPlanEnforcer_CommunityPlan(t *testing.T) {
	pe := NewPlanEnforcer(PlanConfig{
		PlanName:   "community",
		MaxDomains: 1,
		XDPEnabled: false,
		OTAEnabled: false,
		MaxRPM:     60,
	})

	// Valid config within limits
	err := pe.EnforceLimits(RequestedConfig{
		Domains:    1,
		XDPEnabled: false,
		OTAEnabled: false,
		CustomRPM:  60,
	})
	if err != nil {
		t.Fatalf("expected no error for valid config, got: %v", err)
	}

	// Exceeds domain limit
	err = pe.EnforceLimits(RequestedConfig{
		Domains: 2,
	})
	if err == nil {
		t.Fatal("expected error for exceeding domain limit")
	}
	planErr, ok := err.(*PlanEnforcementError)
	if !ok {
		t.Fatalf("expected PlanEnforcementError, got %T", err)
	}
	if len(planErr.Violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(planErr.Violations))
	}
	if planErr.Violations[0].Field != "domains" {
		t.Fatalf("expected violation on 'domains', got '%s'", planErr.Violations[0].Field)
	}

	// XDP not available
	err = pe.EnforceLimits(RequestedConfig{
		Domains:    1,
		XDPEnabled: true,
	})
	if err == nil {
		t.Fatal("expected error for XDP on community plan")
	}

	// OTA not available
	err = pe.EnforceLimits(RequestedConfig{
		Domains:    1,
		OTAEnabled: true,
	})
	if err == nil {
		t.Fatal("expected error for OTA on community plan")
	}

	// RPM exceeds limit
	err = pe.EnforceLimits(RequestedConfig{
		Domains:   1,
		CustomRPM: 120,
	})
	if err == nil {
		t.Fatal("expected error for exceeding RPM limit")
	}
}

func TestPlanEnforcer_ProPlan(t *testing.T) {
	pe := NewPlanEnforcer(PlanConfig{
		PlanName:   "pro",
		MaxDomains: 10,
		XDPEnabled: true,
		OTAEnabled: true,
		MaxRPM:     600,
	})

	// Valid pro config
	err := pe.EnforceLimits(RequestedConfig{
		Domains:    5,
		XDPEnabled: true,
		OTAEnabled: true,
		CustomRPM:  300,
	})
	if err != nil {
		t.Fatalf("expected no error for valid pro config, got: %v", err)
	}

	// Exceeds pro domain limit
	err = pe.EnforceLimits(RequestedConfig{
		Domains: 11,
	})
	if err == nil {
		t.Fatal("expected error for exceeding pro domain limit")
	}
}

func TestPlanEnforcer_MultipleViolations(t *testing.T) {
	pe := NewPlanEnforcer(PlanConfig{
		PlanName:   "community",
		MaxDomains: 1,
		XDPEnabled: false,
		OTAEnabled: false,
		MaxRPM:     60,
	})

	// Multiple violations at once
	err := pe.EnforceLimits(RequestedConfig{
		Domains:    5,
		XDPEnabled: true,
		OTAEnabled: true,
		CustomRPM:  1000,
	})
	if err == nil {
		t.Fatal("expected error for multiple violations")
	}
	planErr, ok := err.(*PlanEnforcementError)
	if !ok {
		t.Fatalf("expected PlanEnforcementError, got %T", err)
	}
	if len(planErr.Violations) != 4 {
		t.Fatalf("expected 4 violations, got %d", len(planErr.Violations))
	}
}

func TestPlanEnforcer_UpdatePlan(t *testing.T) {
	pe := NewPlanEnforcer(PlanConfig{
		PlanName:   "community",
		MaxDomains: 1,
		XDPEnabled: false,
		OTAEnabled: false,
		MaxRPM:     60,
	})

	// Should fail with community limits
	err := pe.EnforceLimits(RequestedConfig{
		Domains:    5,
		XDPEnabled: true,
	})
	if err == nil {
		t.Fatal("expected error with community plan")
	}

	// Upgrade to pro
	pe.UpdatePlan(PlanConfig{
		PlanName:   "pro",
		MaxDomains: 10,
		XDPEnabled: true,
		OTAEnabled: true,
		MaxRPM:     600,
	})

	// Same config should now pass
	err = pe.EnforceLimits(RequestedConfig{
		Domains:    5,
		XDPEnabled: true,
	})
	if err != nil {
		t.Fatalf("expected no error after upgrade to pro, got: %v", err)
	}
}

func TestTrafficAnalyzer_SetPlanThreshold(t *testing.T) {
	baseTime := time.Now()
	currentTime := baseTime

	ta := NewTrafficAnalyzer(TrafficAnalyzerConfig{
		Window:             10 * time.Second,
		PlanThreshold:      100,
		EmergencyThreshold: 5.0,
		NowFunc:            func() time.Time { return currentTime },
	})

	ta.RecordSample(600) // Above 5x100=500

	if !ta.IsEmergency() {
		t.Fatal("should be emergency at 600 rps with threshold 100")
	}

	// Increase threshold
	ta.SetPlanThreshold(200) // 5x200=1000

	if ta.IsEmergency() {
		t.Fatal("should not be emergency at 600 rps with threshold 200 (5x=1000)")
	}
}

func TestTrafficAnalyzer_SampleCount(t *testing.T) {
	baseTime := time.Now()
	currentTime := baseTime

	ta := NewTrafficAnalyzer(TrafficAnalyzerConfig{
		Window:        10 * time.Second,
		PlanThreshold: 100,
		NowFunc:       func() time.Time { return currentTime },
	})

	if ta.SampleCount() != 0 {
		t.Fatal("expected 0 samples initially")
	}

	ta.RecordSample(100)
	currentTime = baseTime.Add(1 * time.Second)
	ta.RecordSample(200)

	if ta.SampleCount() != 2 {
		t.Fatalf("expected 2 samples, got %d", ta.SampleCount())
	}

	// Advance past window for both samples (t=12s, window is 10s)
	// windowStart = t=12s - 10s = t=2s
	// sample at t=0 (< t=2s) → pruned
	// sample at t=1s (< t=2s) → pruned
	// sample at t=12s → kept
	currentTime = baseTime.Add(12 * time.Second)
	ta.RecordSample(300)

	// Both old samples should be pruned, only the new one remains
	if ta.SampleCount() != 1 {
		t.Fatalf("expected 1 sample after pruning, got %d", ta.SampleCount())
	}
}
