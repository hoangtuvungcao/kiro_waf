package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServeBranded502(t *testing.T) {
	w := httptest.NewRecorder()
	ServeBranded502(w)

	if w.Code != 502 {
		t.Fatalf("expected status 502, got %d", w.Code)
	}

	body := w.Body.String()

	// Check content type
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected Content-Type text/html, got %q", ct)
	}

	// Check Vietnamese text is present
	if !strings.Contains(body, "Máy chủ tạm thời không khả dụng") {
		t.Error("expected Vietnamese error title in 502 page")
	}

	// Check Kiro branding
	if !strings.Contains(body, "Kiro WAF") {
		t.Error("expected Kiro WAF branding in 502 page")
	}

	// Check no external dependencies
	if strings.Contains(body, "http://") || strings.Contains(body, "https://") {
		t.Error("502 page should not contain external URLs")
	}

	// Check dark theme elements
	if !strings.Contains(body, "color-scheme: dark") {
		t.Error("expected dark color scheme in 502 page")
	}
}

func TestBranded502HTMLConstant(t *testing.T) {
	if Branded502HTML == "" {
		t.Fatal("Branded502HTML constant should not be empty")
	}
	if !strings.Contains(Branded502HTML, "502") {
		t.Error("Branded502HTML should contain error code 502")
	}
	if !strings.Contains(Branded502HTML, "lang=\"vi\"") {
		t.Error("Branded502HTML should have Vietnamese language attribute")
	}
}

func TestLockdownManager_NewLockdownManager(t *testing.T) {
	adminIPs := []string{"192.168.1.1", "10.0.0.1"}
	lm := NewLockdownManager(adminIPs)

	if lm == nil {
		t.Fatal("NewLockdownManager returned nil")
	}
	if lm.IsLocked() {
		t.Error("new LockdownManager should not be locked")
	}
	if len(lm.adminIPs) != 2 {
		t.Errorf("expected 2 admin IPs, got %d", len(lm.adminIPs))
	}
}

func TestLockdownManager_LockUnlock(t *testing.T) {
	lm := NewLockdownManager(nil)

	// Initially unlocked
	if lm.IsLocked() {
		t.Error("should start unlocked")
	}

	// Lock
	lm.Lock("test_reason")
	if !lm.IsLocked() {
		t.Error("should be locked after Lock()")
	}

	// Check lock info
	locked, reason, lockTime, _ := lm.GetLockInfo()
	if !locked {
		t.Error("GetLockInfo should report locked")
	}
	if reason != "test_reason" {
		t.Errorf("expected reason 'test_reason', got %q", reason)
	}
	if lockTime.IsZero() {
		t.Error("lockTime should not be zero")
	}

	// Unlock
	lm.Unlock()
	if lm.IsLocked() {
		t.Error("should be unlocked after Unlock()")
	}
}

func TestLockdownManager_IsAdminIP(t *testing.T) {
	lm := NewLockdownManager([]string{"192.168.1.1", "10.0.0.5"})

	if !lm.IsAdminIP("192.168.1.1") {
		t.Error("192.168.1.1 should be admin IP")
	}
	if !lm.IsAdminIP("10.0.0.5") {
		t.Error("10.0.0.5 should be admin IP")
	}
	if lm.IsAdminIP("1.2.3.4") {
		t.Error("1.2.3.4 should not be admin IP")
	}
	if lm.IsAdminIP("") {
		t.Error("empty string should not be admin IP")
	}
}

func TestLockdownManager_RecordHeartbeatFailure_LocksAfter3(t *testing.T) {
	lm := NewLockdownManager(nil)

	// First failure - not locked yet
	lm.RecordHeartbeatFailure()
	if lm.IsLocked() {
		t.Error("should not lock after 1 failure")
	}

	// Second failure - not locked yet
	lm.RecordHeartbeatFailure()
	if lm.IsLocked() {
		t.Error("should not lock after 2 failures")
	}

	// Third failure - should lock
	lm.RecordHeartbeatFailure()
	if !lm.IsLocked() {
		t.Error("should lock after 3 consecutive failures")
	}

	// Check reason
	_, reason, _, failures := lm.GetLockInfo()
	if reason != "heartbeat_failed_3_consecutive" {
		t.Errorf("expected reason 'heartbeat_failed_3_consecutive', got %q", reason)
	}
	if failures != 3 {
		t.Errorf("expected 3 failures, got %d", failures)
	}
}

func TestLockdownManager_RecordHeartbeatSuccess_Resets(t *testing.T) {
	lm := NewLockdownManager(nil)

	// Accumulate 3 failures to lock
	lm.RecordHeartbeatFailure()
	lm.RecordHeartbeatFailure()
	lm.RecordHeartbeatFailure()

	if !lm.IsLocked() {
		t.Fatal("should be locked after 3 failures")
	}

	// Success should unlock and reset counter
	lm.RecordHeartbeatSuccess()
	if lm.IsLocked() {
		t.Error("should be unlocked after heartbeat success")
	}

	// Verify counter was reset - need 3 more failures to lock again
	lm.RecordHeartbeatFailure()
	lm.RecordHeartbeatFailure()
	if lm.IsLocked() {
		t.Error("should not be locked after only 2 failures post-reset")
	}

	lm.RecordHeartbeatFailure()
	if !lm.IsLocked() {
		t.Error("should lock again after 3 new consecutive failures")
	}
}

func TestLockdownManager_SuccessResetsCounter(t *testing.T) {
	lm := NewLockdownManager(nil)

	// 2 failures then success
	lm.RecordHeartbeatFailure()
	lm.RecordHeartbeatFailure()
	lm.RecordHeartbeatSuccess()

	// Should need 3 more failures to lock
	lm.RecordHeartbeatFailure()
	lm.RecordHeartbeatFailure()
	if lm.IsLocked() {
		t.Error("should not lock - counter was reset by success")
	}

	lm.RecordHeartbeatFailure()
	if !lm.IsLocked() {
		t.Error("should lock after 3 consecutive failures")
	}
}

func TestLockdownManager_EmptyAdminIPs(t *testing.T) {
	lm := NewLockdownManager([]string{})

	if lm.IsAdminIP("192.168.1.1") {
		t.Error("no IPs should be admin when list is empty")
	}
}

func TestLockdownManager_NilAdminIPs(t *testing.T) {
	lm := NewLockdownManager(nil)

	if lm.IsAdminIP("192.168.1.1") {
		t.Error("no IPs should be admin when list is nil")
	}
}

func TestLockdownManager_Suspended(t *testing.T) {
	lm := NewLockdownManager(nil)

	// Initially not suspended
	if lm.IsSuspended() {
		t.Error("should not be suspended initially")
	}

	// Set suspended
	lm.SetSuspended(true)
	if !lm.IsSuspended() {
		t.Error("should be suspended after SetSuspended(true)")
	}

	// Clear suspended
	lm.SetSuspended(false)
	if lm.IsSuspended() {
		t.Error("should not be suspended after SetSuspended(false)")
	}
}

func TestLockdownManager_CachedPlan(t *testing.T) {
	lm := NewLockdownManager(nil)

	// Initially empty
	if plan := lm.GetCachedPlan(); plan != "" {
		t.Errorf("expected empty cached plan, got %q", plan)
	}

	// Set cached plan
	lm.SetCachedPlan("community")
	if plan := lm.GetCachedPlan(); plan != "community" {
		t.Errorf("expected cached plan 'community', got %q", plan)
	}

	// Update cached plan
	lm.SetCachedPlan("pro")
	if plan := lm.GetCachedPlan(); plan != "pro" {
		t.Errorf("expected cached plan 'pro', got %q", plan)
	}
}

func TestLockdownManager_SuspendedIndependentOfLocked(t *testing.T) {
	lm := NewLockdownManager(nil)

	// Suspended and locked are independent states
	lm.SetSuspended(true)
	if lm.IsLocked() {
		t.Error("suspended should not imply locked")
	}

	lm.Lock("test")
	if !lm.IsSuspended() {
		t.Error("locking should not clear suspended")
	}
	if !lm.IsLocked() {
		t.Error("should be locked")
	}

	lm.Unlock()
	if !lm.IsSuspended() {
		t.Error("unlocking should not clear suspended")
	}

	lm.SetSuspended(false)
	if lm.IsSuspended() {
		t.Error("should not be suspended after clearing")
	}
}

func TestServeSuspendedPage(t *testing.T) {
	w := httptest.NewRecorder()
	ServeSuspendedPage(w)

	if w.Code != 403 {
		t.Fatalf("expected status 403, got %d", w.Code)
	}

	body := w.Body.String()

	// Check content type
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected Content-Type text/html, got %q", ct)
	}

	// Check Vietnamese text is present
	if !strings.Contains(body, "Dịch vụ bị tạm ngưng") {
		t.Error("expected Vietnamese suspension title in page")
	}

	// Check Kiro branding
	if !strings.Contains(body, "Kiro WAF") {
		t.Error("expected Kiro WAF branding in suspended page")
	}

	// Check suspended status indicator
	if !strings.Contains(body, "Suspended") {
		t.Error("expected 'Suspended' text in page")
	}

	// Check dark theme elements
	if !strings.Contains(body, "color-scheme: dark") {
		t.Error("expected dark color scheme in suspended page")
	}
}
