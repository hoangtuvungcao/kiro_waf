// Feature: waf-system-overhaul, Property 2: Hold Time Validation
// **Validates: Requirements 1.2**
//
// For any challenge với IssuedAt timestamp và thời điểm xác minh verifyAt,
// hàm hold verification SHALL chấp nhận khi verifyAt - IssuedAt >= holdSeconds (2 giây)
// và SHALL từ chối khi verifyAt - IssuedAt < holdSeconds.
package property

import (
	"testing"
	"time"

	"kiro_waf/internal/client/challenge"

	"pgregory.net/rapid"
)

// TestHoldTime_PositiveCase verifies that for any random issuedAt and holdSeconds,
// when verifyAt - issuedAt >= holdSeconds, ValidHold returns true.
func TestHoldTime_PositiveCase(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random base timestamp (Unix seconds in a reasonable range)
		baseUnix := rapid.Int64Range(1_000_000_000, 2_000_000_000).Draw(t, "baseUnix")
		issuedAt := time.Unix(baseUnix, 0).UTC()

		// Generate holdSeconds (1 to 30 seconds)
		holdSeconds := rapid.IntRange(1, 30).Draw(t, "holdSeconds")

		// Generate elapsed time that is >= holdSeconds (in milliseconds for precision)
		// elapsedMs is at least holdSeconds*1000 ms, up to holdSeconds*1000 + 60000 ms
		minElapsedMs := int64(holdSeconds) * 1000
		elapsedMs := rapid.Int64Range(minElapsedMs, minElapsedMs+60000).Draw(t, "elapsedMs")
		verifyAt := issuedAt.Add(time.Duration(elapsedMs) * time.Millisecond)

		// Property: ValidHold MUST return true when elapsed >= holdSeconds
		if !challenge.ValidHold(issuedAt, verifyAt, holdSeconds) {
			t.Fatalf("ValidHold returned false but elapsed (%v) >= holdSeconds (%d): issuedAt=%v, verifyAt=%v",
				verifyAt.Sub(issuedAt), holdSeconds, issuedAt, verifyAt)
		}
	})
}

// TestHoldTime_NegativeCase verifies that for any random issuedAt and holdSeconds,
// when verifyAt - issuedAt < holdSeconds, ValidHold returns false.
func TestHoldTime_NegativeCase(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random base timestamp (Unix seconds in a reasonable range)
		baseUnix := rapid.Int64Range(1_000_000_000, 2_000_000_000).Draw(t, "baseUnix")
		issuedAt := time.Unix(baseUnix, 0).UTC()

		// Generate holdSeconds (2 to 30 seconds) - minimum 2 to ensure we can generate elapsed < holdSeconds
		holdSeconds := rapid.IntRange(2, 30).Draw(t, "holdSeconds")

		// Generate elapsed time that is strictly < holdSeconds (in milliseconds)
		// elapsedMs is from 0 to (holdSeconds*1000 - 1) ms
		maxElapsedMs := int64(holdSeconds)*1000 - 1
		elapsedMs := rapid.Int64Range(0, maxElapsedMs).Draw(t, "elapsedMs")
		verifyAt := issuedAt.Add(time.Duration(elapsedMs) * time.Millisecond)

		// Property: ValidHold MUST return false when elapsed < holdSeconds
		if challenge.ValidHold(issuedAt, verifyAt, holdSeconds) {
			t.Fatalf("ValidHold returned true but elapsed (%v) < holdSeconds (%d): issuedAt=%v, verifyAt=%v",
				verifyAt.Sub(issuedAt), holdSeconds, issuedAt, verifyAt)
		}
	})
}
