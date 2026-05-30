package monitor

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewXDPChecker(t *testing.T) {
	alertCalled := false
	checker := NewXDPChecker(XDPCheckerConfig{
		InterfaceName: "eth0",
		XDPObjectPath: "/usr/lib/kiro/xdp_filter.o",
		AlertFunc: func(alertType, message string) {
			alertCalled = true
		},
	})

	if checker.InterfaceName != "eth0" {
		t.Errorf("InterfaceName = %q, want %q", checker.InterfaceName, "eth0")
	}
	if checker.XDPObjectPath != "/usr/lib/kiro/xdp_filter.o" {
		t.Errorf("XDPObjectPath = %q, want %q", checker.XDPObjectPath, "/usr/lib/kiro/xdp_filter.o")
	}
	if checker.maxReloadFailures != 3 {
		t.Errorf("maxReloadFailures = %d, want 3", checker.maxReloadFailures)
	}
	if checker.reloadCooldown != 60*time.Second {
		t.Errorf("reloadCooldown = %v, want 60s", checker.reloadCooldown)
	}

	// Verify alert function is set
	checker.alertFunc("test", "test message")
	if !alertCalled {
		t.Error("alertFunc was not called")
	}
}

func TestNewXDPCheckerDefaultAlert(t *testing.T) {
	// Test that a nil AlertFunc gets a default logger
	checker := NewXDPChecker(XDPCheckerConfig{
		InterfaceName: "eth0",
		XDPObjectPath: "/usr/lib/kiro/xdp_filter.o",
	})

	// Should not panic
	checker.alertFunc("test", "test message")
}

func TestXDPCheckerCheckXDP_NoInterface(t *testing.T) {
	checker := NewXDPChecker(XDPCheckerConfig{
		InterfaceName: "", // No interface configured
		XDPObjectPath: "/usr/lib/kiro/xdp_filter.o",
	})

	ctx := context.Background()
	status := checker.CheckXDP(ctx)

	if status.Attached {
		t.Error("CheckXDP with empty interface should report not attached")
	}
	if status.Stats != nil {
		t.Error("Stats should be nil when not attached")
	}
}

func TestXDPCheckerCheckXDP_NonexistentInterface(t *testing.T) {
	checker := NewXDPChecker(XDPCheckerConfig{
		InterfaceName: "nonexistent_iface_xyz",
		XDPObjectPath: "/usr/lib/kiro/xdp_filter.o",
	})

	ctx := context.Background()
	status := checker.CheckXDP(ctx)

	// Should report not attached for a non-existent interface
	if status.Attached {
		t.Error("CheckXDP with non-existent interface should report not attached")
	}
}

func TestXDPCheckerHandleDetached_NoInterface(t *testing.T) {
	checker := NewXDPChecker(XDPCheckerConfig{
		InterfaceName: "",
		XDPObjectPath: "/usr/lib/kiro/xdp_filter.o",
	})

	ctx := context.Background()
	err := checker.HandleDetached(ctx)

	if err == nil {
		t.Error("HandleDetached with empty interface should return error")
	}
}

func TestXDPCheckerHandleDetached_NoObjectPath(t *testing.T) {
	checker := NewXDPChecker(XDPCheckerConfig{
		InterfaceName: "eth0",
		XDPObjectPath: "",
	})

	ctx := context.Background()
	err := checker.HandleDetached(ctx)

	if err == nil {
		t.Error("HandleDetached with empty object path should return error")
	}
}

func TestXDPCheckerReloadFailureEscalation(t *testing.T) {
	var alertCount int32
	checker := NewXDPChecker(XDPCheckerConfig{
		InterfaceName: "nonexistent_iface_xyz",
		XDPObjectPath: "/nonexistent/path/xdp_filter.o",
		AlertFunc: func(alertType, message string) {
			atomic.AddInt32(&alertCount, 1)
			if alertType != "xdp_reload_failed" {
				t.Errorf("alert type = %q, want %q", alertType, "xdp_reload_failed")
			}
		},
	})

	ctx := context.Background()

	// First 3 failures should trigger escalation alert
	for i := 0; i < 3; i++ {
		err := checker.HandleDetached(ctx)
		if err == nil {
			t.Fatalf("HandleDetached attempt %d should fail", i+1)
		}
	}

	if atomic.LoadInt32(&alertCount) != 1 {
		t.Errorf("alert count = %d, want 1 (triggered on 3rd failure)", atomic.LoadInt32(&alertCount))
	}

	if checker.ReloadFailureCount() != 3 {
		t.Errorf("ReloadFailureCount = %d, want 3", checker.ReloadFailureCount())
	}
}

func TestXDPCheckerReloadCooldown(t *testing.T) {
	checker := NewXDPChecker(XDPCheckerConfig{
		InterfaceName: "nonexistent_iface_xyz",
		XDPObjectPath: "/nonexistent/path/xdp_filter.o",
		AlertFunc:     func(alertType, message string) {},
	})
	// Set a very short cooldown for testing
	checker.reloadCooldown = 50 * time.Millisecond

	ctx := context.Background()

	// Trigger 3 failures to enter cooldown
	for i := 0; i < 3; i++ {
		_ = checker.HandleDetached(ctx)
	}

	// Next attempt should be in cooldown
	err := checker.HandleDetached(ctx)
	if err == nil {
		t.Error("HandleDetached during cooldown should return error")
	}

	// Wait for cooldown to expire
	time.Sleep(60 * time.Millisecond)

	// After cooldown, should attempt reload again (will fail due to nonexistent interface)
	err = checker.HandleDetached(ctx)
	if err == nil {
		t.Error("HandleDetached after cooldown should still fail (bad interface)")
	}

	// But the failure count should have been reset and started fresh
	if checker.ReloadFailureCount() != 1 {
		t.Errorf("ReloadFailureCount after cooldown reset = %d, want 1", checker.ReloadFailureCount())
	}
}

func TestParseXDPStats_Empty(t *testing.T) {
	stats := parseXDPStats("")
	if stats != nil {
		t.Error("parseXDPStats(\"\") should return nil")
	}

	stats = parseXDPStats("[]")
	if stats != nil {
		t.Error("parseXDPStats(\"[]\") should return nil")
	}
}

func TestParseXDPStats_NonEmpty(t *testing.T) {
	stats := parseXDPStats(`[{"key":0,"value":100}]`)
	if stats == nil {
		t.Error("parseXDPStats with data should return non-nil stats")
	}
}
