// Bug Condition Exploration Test - Challenge/Hold Verify Cookie Gating & Loop Bypass
// **Validates: Requirements 1.1, 1.2, 1.3**
//
// These tests encode the EXPECTED (correct) behavior. They are expected to FAIL on
// unfixed code, confirming the bugs exist. After the fix is applied, they should PASS.
//
// Bug 1: handleChallengeVerify sets cookie unconditionally (even on failed verification)
// Bug 2: handleHoldVerify sets cookie unconditionally (even on failed verification)
// Bug 3: Loop bypass proxies without setting a cookie (infinite loop)
package client

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kiro_waf/internal/client/challenge"
	"kiro_waf/internal/client/cookie"

	"pgregory.net/rapid"
)

// TestBugCondition_ChallengeVerify_NoCookieOnInvalidNonce tests that handleChallengeVerify
// does NOT set a cookie when the verification fails (invalid/expired nonce).
//
// Bug Condition: handleChallengeVerify always sets cookie before checking verification.
// Expected Behavior: cookie set ONLY when VerifyChallenge returns true.
//
// **Validates: Requirements 1.1**
func TestBugCondition_ChallengeVerify_NoCookieOnInvalidNonce(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random invalid nonce
		invalidNonce := rapid.String().Draw(t, "invalidNonce")
		clientIP := rapid.StringMatching(`^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$`).Draw(t, "clientIP")

		// Setup proxy handler with minimal dependencies
		store := challenge.NewStore()
		cookieMgr := cookie.NewHMACCookieManager()
		secret := []byte("test-secret-key-for-bugfix-test!")

		handler := NewProxyHandler(
			ProxyConfig{
				BackendURL:   "http://127.0.0.1:9999",
				CookieSecret: secret,
				CookieTTL:    5 * time.Minute,
				Difficulty:   4,
				HoldSeconds:  2,
				ChallengeTTL: 90 * time.Second,
			},
			cookieMgr,
			nil, // rateLimiter not needed for this test
			nil, // banEngine not needed for this test
			store,
		)

		// Create a POST request to /__kiro/challenge/verify with invalid token/nonce
		body, _ := json.Marshal(map[string]string{
			"token": "nonexistent-token-that-does-not-exist",
			"nonce": invalidNonce,
		})
		req := httptest.NewRequest(http.MethodPost, "/__kiro/challenge/verify", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = clientIP + ":12345"

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		// Property: When verification fails (invalid token), NO Set-Cookie header should be present
		resp := rec.Result()
		cookies := resp.Cookies()
		for _, c := range cookies {
			if c.Name == "kiro_access" {
				t.Fatalf("BUG CONFIRMED: Set-Cookie 'kiro_access' header present despite failed challenge verification (invalid nonce=%q, ip=%s). Cookie should only be set on successful verification.", invalidNonce, clientIP)
			}
		}
	})
}

// TestBugCondition_HoldVerify_NoCookieOnInsufficientDuration tests that handleHoldVerify
// does NOT set a cookie when the hold duration is insufficient.
//
// Bug Condition: handleHoldVerify always sets cookie before checking verification.
// Expected Behavior: cookie set ONLY when VerifyHold returns true.
//
// **Validates: Requirements 1.2**
func TestBugCondition_HoldVerify_NoCookieOnInsufficientDuration(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		clientIP := rapid.StringMatching(`^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$`).Draw(t, "clientIP")

		// Setup proxy handler
		store := challenge.NewStore()
		cookieMgr := cookie.NewHMACCookieManager()
		secret := []byte("test-secret-key-for-bugfix-test!")

		handler := NewProxyHandler(
			ProxyConfig{
				BackendURL:   "http://127.0.0.1:9999",
				CookieSecret: secret,
				CookieTTL:    5 * time.Minute,
				Difficulty:   4,
				HoldSeconds:  2,
				ChallengeTTL: 90 * time.Second,
			},
			cookieMgr,
			nil,
			nil,
			store,
		)

		// Issue a challenge token but DON'T wait the required hold duration
		// Use an invalid/nonexistent token to simulate failed verification
		body, _ := json.Marshal(map[string]string{
			"token": "nonexistent-hold-token-invalid",
		})
		req := httptest.NewRequest(http.MethodPost, "/__kiro/hold/verify", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = clientIP + ":12345"

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		// Property: When hold verification fails, NO Set-Cookie header should be present
		resp := rec.Result()
		cookies := resp.Cookies()
		for _, c := range cookies {
			if c.Name == "kiro_access" {
				t.Fatalf("BUG CONFIRMED: Set-Cookie 'kiro_access' header present despite failed hold verification (ip=%s). Cookie should only be set on successful verification.", clientIP)
			}
		}
	})
}

// TestBugCondition_LoopBypass_SetsCookie tests that when the LoopDetector triggers
// a bypass (same challenge >3 times in 10s), the response DOES contain a Set-Cookie header.
//
// Bug Condition: loop bypass proxies request without setting cookie, causing infinite loop.
// Expected Behavior: short-lived access cookie set before proxying on loop bypass.
//
// **Validates: Requirements 1.3**
func TestBugCondition_LoopBypass_SetsCookie(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		clientIP := rapid.StringMatching(`^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$`).Draw(t, "clientIP")

		// Setup a backend that just returns 200
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("backend response"))
		}))
		defer backend.Close()

		store := challenge.NewStore()
		cookieMgr := cookie.NewHMACCookieManager()
		secret := []byte("test-secret-key-for-bugfix-test!")
		loopDetector := challenge.NewLoopDetector()

		handler := NewProxyHandler(
			ProxyConfig{
				BackendURL:   backend.URL,
				CookieSecret: secret,
				CookieTTL:    5 * time.Minute,
				Difficulty:   4,
				HoldSeconds:  2,
				ChallengeTTL: 90 * time.Second,
			},
			cookieMgr,
			nil,
			nil,
			store,
		)
		// Inject the loop detector
		handler.loopDetector = loopDetector
		// Force challenge level to 2 (PoW) so loop detection is relevant
		handler.challengeAll = true

		// Record 4 challenges for this IP to trigger loop bypass
		for i := 0; i < 4; i++ {
			loopDetector.Record(clientIP, "transparent")
		}

		// Now make a request - the loop detector should trigger bypass
		req := httptest.NewRequest(http.MethodGet, "/some-page", nil)
		req.RemoteAddr = clientIP + ":12345"
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		// Property: When loop bypass is triggered, a Set-Cookie header MUST be present
		// to prevent infinite re-triggering of the challenge
		resp := rec.Result()
		cookies := resp.Cookies()
		hasCookie := false
		for _, c := range cookies {
			if c.Name == "kiro_access" {
				hasCookie = true
				break
			}
		}
		if !hasCookie {
			t.Fatalf("BUG CONFIRMED: No Set-Cookie 'kiro_access' header on loop bypass (ip=%s). Without a cookie, the next request will re-trigger the challenge creating an infinite loop.", clientIP)
		}
	})
}
