// Feature: waf-system-overhaul, Property 11: Periodic Cleanup Removes Exactly Expired Entries
// **Validates: Requirements 11.7**
//
// For any set of rate-limit entries with various timestamps, running cleanup SHALL
// remove exactly those entries whose timestamp is older than the configured window
// duration, and SHALL retain all entries within the window. Similarly for challenge
// tokens with their 60-second TTL.
package property

import (
	"fmt"
	"testing"
	"time"

	"kiro_waf/internal/client/challenge"
	"kiro_waf/internal/client/ratelimit"

	"pgregory.net/rapid"
)

// --- Property 11: Periodic Cleanup Removes Exactly Expired Entries ---

// TestCleanup_RateLimitRemovesExactlyExpiredEntries verifies that for any set of
// rate-limit entries with various timestamps, Cleanup() removes exactly those entries
// whose timestamp is older than the configured window (120s), and retains all entries
// within the window.
func TestCleanup_RateLimitRemovesExactlyExpiredEntries(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a window duration between 30s and 300s (covers the 120s default)
		windowSec := rapid.IntRange(30, 300).Draw(t, "windowSec")
		windowDuration := time.Duration(windowSec) * time.Second

		config := ratelimit.LimiterConfig{
			SoftThreshold:   1000, // High threshold so we don't hit limits
			HardThreshold:   2000,
			SubnetThreshold: 5000,
			WindowDuration:  windowDuration,
		}
		limiter := ratelimit.NewSlidingWindowLimiter(config)

		// Generate number of IPs with "old" entries (should be removed)
		numExpiredIPs := rapid.IntRange(0, 10).Draw(t, "numExpiredIPs")
		// Generate number of IPs with "recent" entries (should be retained)
		numValidIPs := rapid.IntRange(0, 10).Draw(t, "numValidIPs")

		baseTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

		// Track which IPs should be expired vs retained
		expiredIPs := make([]string, 0, numExpiredIPs)
		validIPs := make([]string, 0, numValidIPs)

		// Insert "old" entries: timestamps older than window
		for i := 0; i < numExpiredIPs; i++ {
			ip := fmt.Sprintf("10.%d.%d.%d", (i/256)%256, (i/16)%256, i%256+1)
			// Place entry at a time that is beyond the window from our cleanup time
			ageSec := rapid.IntRange(windowSec+1, windowSec+600).Draw(t, fmt.Sprintf("expiredAge_%d", i))
			entryTime := baseTime.Add(-time.Duration(ageSec) * time.Second)

			limiter.SetNowFunc(func() time.Time { return entryTime })
			limiter.RecordRequest(ip)
			expiredIPs = append(expiredIPs, ip)
		}

		// Insert "recent" entries: timestamps within the window
		for i := 0; i < numValidIPs; i++ {
			ip := fmt.Sprintf("172.%d.%d.%d", (i/256)%256, (i/16)%256, i%256+1)
			// Place entry within the window (0 to windowSec-1 seconds ago from cleanup time)
			ageSec := rapid.IntRange(0, windowSec-1).Draw(t, fmt.Sprintf("validAge_%d", i))
			entryTime := baseTime.Add(-time.Duration(ageSec) * time.Second)

			limiter.SetNowFunc(func() time.Time { return entryTime })
			limiter.RecordRequest(ip)
			validIPs = append(validIPs, ip)
		}

		// Set "now" to baseTime and run cleanup
		limiter.SetNowFunc(func() time.Time { return baseTime })
		limiter.Cleanup()

		// Property: all expired IPs should be removed
		for _, ip := range expiredIPs {
			if limiter.HasIP(ip) {
				t.Fatalf("expired IP %q should have been removed by cleanup (window=%v)", ip, windowDuration)
			}
		}

		// Property: all valid IPs should be retained
		for _, ip := range validIPs {
			if !limiter.HasIP(ip) {
				t.Fatalf("valid IP %q should have been retained by cleanup (window=%v)", ip, windowDuration)
			}
		}
	})
}

// TestCleanup_ChallengeStoreRemovesExactlyExpiredTokens verifies that for any set
// of challenge tokens with various expiry times, Cleanup() removes exactly those
// tokens that have expired (past their TTL), and retains all tokens still within TTL.
func TestCleanup_ChallengeStoreRemovesExactlyExpiredTokens(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		store := challenge.NewStore()

		// Generate number of expired tokens
		numExpired := rapid.IntRange(0, 15).Draw(t, "numExpired")
		// Generate number of valid tokens
		numValid := rapid.IntRange(0, 15).Draw(t, "numValid")

		baseTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

		expiredTokens := make([]string, 0, numExpired)
		validTokens := make([]string, 0, numValid)

		// Insert tokens that should be expired at cleanup time
		for i := 0; i < numExpired; i++ {
			// TTL between 10s and 120s
			ttlSec := rapid.IntRange(10, 120).Draw(t, fmt.Sprintf("expiredTTL_%d", i))
			ttl := time.Duration(ttlSec) * time.Second

			// Issue the token far enough in the past that it's expired by baseTime
			issueOffset := rapid.IntRange(ttlSec+1, ttlSec+600).Draw(t, fmt.Sprintf("expiredOffset_%d", i))
			issueTime := baseTime.Add(-time.Duration(issueOffset) * time.Second)

			clientIP := fmt.Sprintf("192.168.%d.%d", i/256, i%256+1)
			entry := store.IssueAt(clientIP, 4, ttl, issueTime)
			expiredTokens = append(expiredTokens, entry.Token)
		}

		// Insert tokens that should still be valid at cleanup time
		for i := 0; i < numValid; i++ {
			// TTL between 30s and 120s
			ttlSec := rapid.IntRange(30, 120).Draw(t, fmt.Sprintf("validTTL_%d", i))
			ttl := time.Duration(ttlSec) * time.Second

			// Issue the token recently enough that it's still valid at baseTime
			issueOffset := rapid.IntRange(0, ttlSec-1).Draw(t, fmt.Sprintf("validOffset_%d", i))
			issueTime := baseTime.Add(-time.Duration(issueOffset) * time.Second)

			clientIP := fmt.Sprintf("10.0.%d.%d", i/256, i%256+1)
			entry := store.IssueAt(clientIP, 4, ttl, issueTime)
			validTokens = append(validTokens, entry.Token)
		}

		// Run cleanup at baseTime
		store.CleanupAt(baseTime)

		// Property: all expired tokens should be removed
		for _, token := range expiredTokens {
			if store.Has(token) {
				t.Fatalf("expired token %q should have been removed by cleanup", token)
			}
		}

		// Property: all valid tokens should be retained
		for _, token := range validTokens {
			if !store.Has(token) {
				t.Fatalf("valid token %q should have been retained by cleanup", token)
			}
		}
	})
}

// TestCleanup_RateLimitSubnetConsistency verifies that cleanup also correctly
// handles subnet entries — expired subnet entries are removed, valid ones retained.
func TestCleanup_RateLimitSubnetConsistency(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		windowSec := rapid.IntRange(60, 180).Draw(t, "windowSec")
		windowDuration := time.Duration(windowSec) * time.Second

		config := ratelimit.LimiterConfig{
			SoftThreshold:   1000,
			HardThreshold:   2000,
			SubnetThreshold: 5000,
			WindowDuration:  windowDuration,
		}
		limiter := ratelimit.NewSlidingWindowLimiter(config)

		baseTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

		// Insert an expired entry
		expiredTime := baseTime.Add(-time.Duration(windowSec+10) * time.Second)
		limiter.SetNowFunc(func() time.Time { return expiredTime })
		limiter.RecordRequest("10.0.1.50")

		// Insert a valid entry in a different subnet
		validTime := baseTime.Add(-time.Duration(windowSec/2) * time.Second)
		limiter.SetNowFunc(func() time.Time { return validTime })
		limiter.RecordRequest("10.0.2.50")

		// Run cleanup
		limiter.SetNowFunc(func() time.Time { return baseTime })
		limiter.Cleanup()

		// Property: expired subnet entry removed
		if limiter.HasSubnet("10.0.1.0/24") {
			t.Fatal("expired subnet 10.0.1.0/24 should have been removed")
		}

		// Property: valid subnet entry retained
		if !limiter.HasSubnet("10.0.2.0/24") {
			t.Fatal("valid subnet 10.0.2.0/24 should have been retained")
		}
	})
}
