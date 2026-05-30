package ratelimit

import (
	"sync"
	"testing"
	"time"
)

func TestNewCookieRateLimiter(t *testing.T) {
	config := CookieRateLimiterConfig{
		Threshold: 300,
		Window:    60 * time.Second,
	}
	limiter := NewCookieRateLimiter(config)

	if limiter == nil {
		t.Fatal("NewCookieRateLimiter returned nil")
	}
	if limiter.threshold != 300 {
		t.Errorf("expected threshold=300, got %d", limiter.threshold)
	}
	if limiter.window != 60*time.Second {
		t.Errorf("expected window=60s, got %v", limiter.window)
	}
}

func TestRecordAndCheck_UnderThreshold(t *testing.T) {
	config := CookieRateLimiterConfig{
		Threshold: 5,
		Window:    60 * time.Second,
	}
	limiter := NewCookieRateLimiter(config)

	cookie := "test-cookie-value-abc123"

	// Record 4 requests (under threshold of 5)
	for i := 0; i < 4; i++ {
		if !limiter.RecordAndCheck(cookie) {
			t.Errorf("expected RecordAndCheck=true on request %d (under threshold)", i+1)
		}
	}
}

func TestRecordAndCheck_AtThreshold(t *testing.T) {
	config := CookieRateLimiterConfig{
		Threshold: 5,
		Window:    60 * time.Second,
	}
	limiter := NewCookieRateLimiter(config)

	cookie := "test-cookie-value-abc123"

	// Record exactly 5 requests (at threshold)
	for i := 0; i < 5; i++ {
		if !limiter.RecordAndCheck(cookie) {
			t.Errorf("expected RecordAndCheck=true on request %d (at threshold)", i+1)
		}
	}
}

func TestRecordAndCheck_ExceedsThreshold(t *testing.T) {
	config := CookieRateLimiterConfig{
		Threshold: 5,
		Window:    60 * time.Second,
	}
	limiter := NewCookieRateLimiter(config)

	cookie := "test-cookie-value-abc123"

	// Record 5 requests (at threshold, still OK)
	for i := 0; i < 5; i++ {
		limiter.RecordAndCheck(cookie)
	}

	// 6th request should exceed threshold and revoke
	if limiter.RecordAndCheck(cookie) {
		t.Error("expected RecordAndCheck=false when exceeding threshold")
	}
}

func TestRecordAndCheck_RevokedCookieAlwaysFails(t *testing.T) {
	config := CookieRateLimiterConfig{
		Threshold: 3,
		Window:    60 * time.Second,
	}
	limiter := NewCookieRateLimiter(config)

	cookie := "revoked-cookie"

	// Exceed threshold to trigger revocation
	for i := 0; i < 4; i++ {
		limiter.RecordAndCheck(cookie)
	}

	// Subsequent requests should always fail
	for i := 0; i < 5; i++ {
		if limiter.RecordAndCheck(cookie) {
			t.Errorf("expected RecordAndCheck=false for revoked cookie on attempt %d", i+1)
		}
	}
}

func TestRecordAndCheck_DifferentCookiesIndependent(t *testing.T) {
	config := CookieRateLimiterConfig{
		Threshold: 3,
		Window:    60 * time.Second,
	}
	limiter := NewCookieRateLimiter(config)

	cookie1 := "cookie-alpha"
	cookie2 := "cookie-beta"

	// Exceed threshold for cookie1
	for i := 0; i < 4; i++ {
		limiter.RecordAndCheck(cookie1)
	}

	// cookie1 should be revoked
	if limiter.RecordAndCheck(cookie1) {
		t.Error("expected cookie1 to be revoked")
	}

	// cookie2 should still be allowed
	if !limiter.RecordAndCheck(cookie2) {
		t.Error("expected cookie2 to still be allowed")
	}
}

func TestRecordAndCheck_WindowReset(t *testing.T) {
	config := CookieRateLimiterConfig{
		Threshold: 5,
		Window:    60 * time.Second,
	}
	limiter := NewCookieRateLimiter(config)

	now := time.Now()
	limiter.SetNowFunc(func() time.Time { return now })

	cookie := "window-test-cookie"

	// Record 5 requests (at threshold)
	for i := 0; i < 5; i++ {
		limiter.RecordAndCheck(cookie)
	}

	// Advance time past window
	limiter.SetNowFunc(func() time.Time { return now.Add(61 * time.Second) })

	// Counter should reset, so this should succeed
	if !limiter.RecordAndCheck(cookie) {
		t.Error("expected RecordAndCheck=true after window reset")
	}
}

func TestIsRevoked_NotRevoked(t *testing.T) {
	config := CookieRateLimiterConfig{
		Threshold: 10,
		Window:    60 * time.Second,
	}
	limiter := NewCookieRateLimiter(config)

	cookie := "normal-cookie"
	limiter.RecordAndCheck(cookie)

	if limiter.IsRevoked(cookie) {
		t.Error("expected IsRevoked=false for non-revoked cookie")
	}
}

func TestIsRevoked_Revoked(t *testing.T) {
	config := CookieRateLimiterConfig{
		Threshold: 2,
		Window:    60 * time.Second,
	}
	limiter := NewCookieRateLimiter(config)

	cookie := "bad-cookie"

	// Exceed threshold
	for i := 0; i < 3; i++ {
		limiter.RecordAndCheck(cookie)
	}

	if !limiter.IsRevoked(cookie) {
		t.Error("expected IsRevoked=true for revoked cookie")
	}
}

func TestIsRevoked_UnknownCookie(t *testing.T) {
	config := CookieRateLimiterConfig{
		Threshold: 10,
		Window:    60 * time.Second,
	}
	limiter := NewCookieRateLimiter(config)

	if limiter.IsRevoked("never-seen-cookie") {
		t.Error("expected IsRevoked=false for unknown cookie")
	}
}

func TestCleanup_ExpiredCounters(t *testing.T) {
	config := CookieRateLimiterConfig{
		Threshold: 100,
		Window:    60 * time.Second,
	}
	limiter := NewCookieRateLimiter(config)

	now := time.Now()
	limiter.SetNowFunc(func() time.Time { return now })

	// Record some requests
	limiter.RecordAndCheck("cookie-a")
	limiter.RecordAndCheck("cookie-b")

	// Advance past window
	limiter.SetNowFunc(func() time.Time { return now.Add(61 * time.Second) })

	limiter.Cleanup()

	// Counters should be cleaned up
	limiter.mu.Lock()
	counterCount := len(limiter.counters)
	limiter.mu.Unlock()

	if counterCount != 0 {
		t.Errorf("expected 0 counters after cleanup, got %d", counterCount)
	}
}

func TestCleanup_ExpiredRevocations(t *testing.T) {
	config := CookieRateLimiterConfig{
		Threshold: 2,
		Window:    60 * time.Second,
	}
	limiter := NewCookieRateLimiter(config)

	now := time.Now()
	limiter.SetNowFunc(func() time.Time { return now })

	// Revoke a cookie
	for i := 0; i < 3; i++ {
		limiter.RecordAndCheck("revoke-me")
	}

	if !limiter.IsRevoked("revoke-me") {
		t.Fatal("cookie should be revoked")
	}

	// Advance past 2× window (revocation expiry)
	limiter.SetNowFunc(func() time.Time { return now.Add(121 * time.Second) })

	limiter.Cleanup()

	// Revocation should be cleaned up
	if limiter.IsRevoked("revoke-me") {
		t.Error("expected revocation to be cleaned up after 2× window")
	}
}

func TestCookieCleanup_PartialExpiry(t *testing.T) {
	config := CookieRateLimiterConfig{
		Threshold: 100,
		Window:    60 * time.Second,
	}
	limiter := NewCookieRateLimiter(config)

	now := time.Now()
	limiter.SetNowFunc(func() time.Time { return now })

	// Record old request
	limiter.RecordAndCheck("old-cookie")

	// Record recent request
	limiter.SetNowFunc(func() time.Time { return now.Add(50 * time.Second) })
	limiter.RecordAndCheck("recent-cookie")

	// Advance to t=61s: old-cookie expired, recent-cookie still valid
	limiter.SetNowFunc(func() time.Time { return now.Add(61 * time.Second) })
	limiter.Cleanup()

	limiter.mu.Lock()
	counterCount := len(limiter.counters)
	limiter.mu.Unlock()

	if counterCount != 1 {
		t.Errorf("expected 1 counter after partial cleanup, got %d", counterCount)
	}
}

func TestCookieRateLimiter_ConcurrentAccess(t *testing.T) {
	config := CookieRateLimiterConfig{
		Threshold: 1000,
		Window:    60 * time.Second,
	}
	limiter := NewCookieRateLimiter(config)

	var wg sync.WaitGroup
	numGoroutines := 10
	requestsPerGoroutine := 100

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			cookie := "cookie-" + itoa(id)
			for i := 0; i < requestsPerGoroutine; i++ {
				limiter.RecordAndCheck(cookie)
				limiter.IsRevoked(cookie)
			}
		}(g)
	}

	wg.Wait()
	// Should not panic or deadlock
}

func TestCookieRateLimiter_ConcurrentCleanup(t *testing.T) {
	config := CookieRateLimiterConfig{
		Threshold: 5,
		Window:    60 * time.Second,
	}
	limiter := NewCookieRateLimiter(config)

	var wg sync.WaitGroup

	// Concurrent writes
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			limiter.RecordAndCheck("concurrent-cookie-" + itoa(i))
		}
	}()

	// Concurrent cleanup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			limiter.Cleanup()
		}
	}()

	wg.Wait()
	// Should not panic or deadlock
}

func TestHashCookie_Deterministic(t *testing.T) {
	cookie := "test-cookie-value"
	hash1 := hashCookie(cookie)
	hash2 := hashCookie(cookie)

	if hash1 != hash2 {
		t.Errorf("hashCookie not deterministic: %d != %d", hash1, hash2)
	}
}

func TestHashCookie_DifferentValues(t *testing.T) {
	hash1 := hashCookie("cookie-a")
	hash2 := hashCookie("cookie-b")

	if hash1 == hash2 {
		t.Error("expected different hashes for different cookie values")
	}
}
