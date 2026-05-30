// Feature: waf-system-overhaul, Property 16: Heartbeat Failure Lockdown
// **Validates: Requirements 9.2**
//
// For any chuỗi heartbeat responses, khi có N lần thất bại liên tiếp (N >= 3),
// Client_WAF SHALL chuyển sang trạng thái locked. Khi heartbeat thành công tiếp theo
// xảy ra, trạng thái SHALL chuyển về unlocked.
package client

import (
	"testing"

	"pgregory.net/rapid"
)

// TestHeartbeatLockdown_ConsecutiveFailuresLock verifies that after N >= 3 consecutive
// heartbeat failures, the LockdownManager transitions to locked state.
func TestHeartbeatLockdown_ConsecutiveFailuresLock(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate N >= 3 consecutive failures
		n := rapid.IntRange(3, 50).Draw(t, "consecutiveFailures")

		lm := NewLockdownManager([]string{"127.0.0.1"})

		// Record N consecutive failures
		for i := 0; i < n; i++ {
			lm.RecordHeartbeatFailure()
		}

		// Property: After N >= 3 consecutive failures, IsLocked() MUST return true
		if !lm.IsLocked() {
			t.Fatalf("expected locked state after %d consecutive failures, but IsLocked() returned false", n)
		}
	})
}

// TestHeartbeatLockdown_SuccessAfterLockdownUnlocks verifies that after entering lockdown
// due to consecutive failures, a successful heartbeat transitions back to unlocked state.
func TestHeartbeatLockdown_SuccessAfterLockdownUnlocks(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate N >= 3 consecutive failures to trigger lockdown
		n := rapid.IntRange(3, 50).Draw(t, "consecutiveFailures")

		lm := NewLockdownManager([]string{"127.0.0.1"})

		// Trigger lockdown with N consecutive failures
		for i := 0; i < n; i++ {
			lm.RecordHeartbeatFailure()
		}

		// Sanity check: must be locked
		if !lm.IsLocked() {
			t.Fatalf("precondition failed: expected locked after %d failures", n)
		}

		// Record a successful heartbeat
		lm.RecordHeartbeatSuccess()

		// Property: After a success following lockdown, IsLocked() MUST return false
		if lm.IsLocked() {
			t.Fatalf("expected unlocked state after successful heartbeat following %d consecutive failures, but IsLocked() returned true", n)
		}
	})
}

// TestHeartbeatLockdown_InterspersedSuccessResetsCounter verifies that failures
// interspersed with successes do not trigger lockdown because the counter resets.
func TestHeartbeatLockdown_InterspersedSuccessResetsCounter(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a sequence of events where successes are interspersed
		// such that we never reach 3 consecutive failures.
		// Strategy: generate groups of 1-2 failures each followed by a success.
		numGroups := rapid.IntRange(1, 20).Draw(t, "numGroups")

		lm := NewLockdownManager([]string{"127.0.0.1"})

		for i := 0; i < numGroups; i++ {
			// Each group has 1 or 2 failures (never reaching 3 consecutive)
			failures := rapid.IntRange(1, 2).Draw(t, "failuresInGroup")
			for j := 0; j < failures; j++ {
				lm.RecordHeartbeatFailure()
			}
			// Intersperse with a success to reset the counter
			lm.RecordHeartbeatSuccess()
		}

		// Property: Failures interspersed with successes (never 3 consecutive)
		// MUST NOT trigger lockdown
		if lm.IsLocked() {
			t.Fatalf("expected unlocked state when failures are interspersed with successes (max 2 consecutive failures per group), but IsLocked() returned true")
		}
	})
}
