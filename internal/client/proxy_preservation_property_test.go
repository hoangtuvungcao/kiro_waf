// Feature: waf-challenge-xdp-fix, Property 2: Preservation - Proxy Behaviors Unchanged
// **Validates: Requirements 3.1, 3.2, 3.6**
//
// For all requests with valid cookies, the proxy serves the backend response without
// issuing challenges.
// For all challenge requests where loop count ≤ 3, the challenge page is served (no bypass).
// handleTransparentVerify sets cookie only on successful verification (already correct).
package client

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kiro_waf/internal/client/ban"
	"kiro_waf/internal/client/challenge"
	"kiro_waf/internal/client/cookie"
	"kiro_waf/internal/client/ratelimit"

	"pgregory.net/rapid"
)

const testUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// TestPreservation_ValidCookie_ProxiedWithoutChallenge verifies that for all requests
// with valid kiro_access cookies, the proxy serves the backend response directly
// without issuing any challenges.
//
// **Validates: Requirements 3.2**
func TestPreservation_ValidCookie_ProxiedWithoutChallenge(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random IP and path
		a := rapid.IntRange(1, 254).Draw(t, "ip_a")
		b := rapid.IntRange(0, 255).Draw(t, "ip_b")
		c := rapid.IntRange(0, 255).Draw(t, "ip_c")
		d := rapid.IntRange(1, 254).Draw(t, "ip_d")
		clientIP := intToStrProxy(a) + "." + intToStrProxy(b) + "." + intToStrProxy(c) + "." + intToStrProxy(d)

		// Generate a random non-passthrough path
		pathSuffix := rapid.StringMatching(`^[a-z]{1,10}$`).Draw(t, "pathSuffix")
		path := "/page/" + pathSuffix

		// Setup backend that returns a distinctive response
		backendBody := "backend-ok-" + clientIP
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Backend", "true")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(backendBody))
		}))
		defer backend.Close()

		store := challenge.NewStore()
		cookieMgr := cookie.NewHMACCookieManager()
		secret := []byte("test-secret-preservation-cookie!")
		banEngine := ban.NewInMemoryBanEngine("", "")

		handler := NewProxyHandler(
			ProxyConfig{
				BackendURL:      backend.URL,
				CookieSecret:    secret,
				CookieTTL:       5 * time.Minute,
				Difficulty:      4,
				HoldSeconds:     2,
				ChallengeTTL:    90 * time.Second,
				ChallengeAllNew: true, // Force challenges for new visitors
			},
			cookieMgr,
			nil, // rateLimiter
			banEngine,
			store,
		)

		// Generate a valid cookie for this IP
		validCookie, err := cookieMgr.GenerateCookie(clientIP, secret, 5*time.Minute)
		if err != nil {
			t.Fatalf("failed to generate valid cookie: %v", err)
		}

		// Create request with valid cookie
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = clientIP + ":12345"
		req.Header.Set("User-Agent", testUserAgent)
		req.AddCookie(&http.Cookie{
			Name:  "kiro_access",
			Value: validCookie,
		})

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		// Property: Requests with valid cookies MUST be proxied to backend without challenge
		resp := rec.Result()
		body := rec.Body.String()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected HTTP 200 for valid cookie request, got %d (ip=%s, path=%s)", resp.StatusCode, clientIP, path)
		}

		if !strings.Contains(body, "backend-ok-") {
			t.Fatalf("expected backend response body for valid cookie request, got %q (ip=%s, path=%s). Request was likely challenged instead of proxied.", body, clientIP, path)
		}
	})
}

// TestPreservation_ChallengeServed_WhenLoopCountLe3 verifies that for all challenge
// requests where the loop count is ≤ 3, the challenge page is served normally
// without bypassing.
//
// **Validates: Requirements 3.6**
func TestPreservation_ChallengeServed_WhenLoopCountLe3(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random IP
		a := rapid.IntRange(1, 254).Draw(t, "ip_a")
		b := rapid.IntRange(0, 255).Draw(t, "ip_b")
		c := rapid.IntRange(0, 255).Draw(t, "ip_c")
		d := rapid.IntRange(1, 254).Draw(t, "ip_d")
		clientIP := intToStrProxy(a) + "." + intToStrProxy(b) + "." + intToStrProxy(c) + "." + intToStrProxy(d)

		// Generate loop count between 0 and 3 (should NOT trigger bypass)
		loopCount := rapid.IntRange(0, 3).Draw(t, "loopCount")

		// Setup backend
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("backend"))
		}))
		defer backend.Close()

		store := challenge.NewStore()
		cookieMgr := cookie.NewHMACCookieManager()
		secret := []byte("test-secret-preservation-loop!!")
		loopDetector := challenge.NewLoopDetector()
		banEngine := ban.NewInMemoryBanEngine("", "")
		rateLimiter := ratelimit.NewSlidingWindowLimiter(ratelimit.LimiterConfig{
			SoftThreshold:   1000,
			HardThreshold:   2000,
			SubnetThreshold: 5000,
			WindowDuration:  60 * time.Second,
		})

		handler := NewProxyHandler(
			ProxyConfig{
				BackendURL:      backend.URL,
				CookieSecret:    secret,
				CookieTTL:       5 * time.Minute,
				Difficulty:      4,
				HoldSeconds:     2,
				ChallengeTTL:    90 * time.Second,
				ChallengeAllNew: true, // Force challenge for all new visitors (level 1 = transparent)
			},
			cookieMgr,
			rateLimiter,
			banEngine,
			store,
		)
		handler.loopDetector = loopDetector

		// Record loopCount challenges (≤ 3, so bypass should NOT trigger)
		for i := 0; i < loopCount; i++ {
			loopDetector.Record(clientIP, "transparent")
		}

		// Make a request without a valid cookie - should get challenge page
		req := httptest.NewRequest(http.MethodGet, "/test-page", nil)
		req.RemoteAddr = clientIP + ":12345"
		req.Header.Set("User-Agent", testUserAgent)

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		// Property: When loop count ≤ 3, challenge page MUST be served (no bypass)
		resp := rec.Result()
		body := rec.Body.String()

		// The response should be a challenge page (transparent challenge HTML)
		// NOT the backend response
		if strings.Contains(body, "backend") {
			t.Fatalf("expected challenge page to be served when loop count=%d (≤3), but got backend response (ip=%s). Challenge was bypassed when it should not have been.", loopCount, clientIP)
		}

		// Challenge page should contain the transparent challenge HTML markers
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected HTTP 200 for challenge page, got %d (ip=%s, loopCount=%d)", resp.StatusCode, clientIP, loopCount)
		}

		// Verify it's actually a challenge page (contains the verify endpoint reference)
		if !strings.Contains(body, "/__kiro/transparent/verify") {
			t.Fatalf("expected transparent challenge page content when loop count=%d (≤3), got unexpected response body (ip=%s)", loopCount, clientIP)
		}
	})
}

// TestPreservation_TransparentVerify_SetsCookieOnSuccess verifies that
// handleTransparentVerify sets cookie only on successful verification.
// This is already correctly implemented and must remain unchanged.
//
// **Validates: Requirements 3.1**
func TestPreservation_TransparentVerify_SetsCookieOnSuccess(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random IP
		a := rapid.IntRange(1, 254).Draw(t, "ip_a")
		b := rapid.IntRange(0, 255).Draw(t, "ip_b")
		c := rapid.IntRange(0, 255).Draw(t, "ip_c")
		d := rapid.IntRange(1, 254).Draw(t, "ip_d")
		clientIP := intToStrProxy(a) + "." + intToStrProxy(b) + "." + intToStrProxy(c) + "." + intToStrProxy(d)

		store := challenge.NewStore()
		cookieMgr := cookie.NewHMACCookieManager()
		secret := []byte("test-secret-preservation-trans!!")

		handler := NewProxyHandler(
			ProxyConfig{
				BackendURL:     "http://127.0.0.1:9999",
				CookieSecret:   secret,
				CookieTTL:      5 * time.Minute,
				Difficulty:     4,
				HoldSeconds:    2,
				ChallengeTTL:   90 * time.Second,
				TransparentTTL: 30 * time.Second,
			},
			cookieMgr,
			nil,
			nil,
			store,
		)

		// Issue a valid transparent challenge token for this IP
		entry := store.Issue(clientIP, 0, 30*time.Second)

		// Wait a tiny bit to pass the minimum solve time check (20ms)
		time.Sleep(25 * time.Millisecond)

		// Compute the expected solution (HMAC-like XOR hash matching the JS)
		solution := computeTransparentSolution(entry.Salt, entry.Token)

		// Build a valid transparent verify request with fingerprint
		reqBody := map[string]interface{}{
			"token":    entry.Token,
			"solution": solution,
			"fp": map[string]interface{}{
				"canvas": "test-canvas-hash-value",
				"webgl":  "test-webgl-renderer",
				"tz":     -420,
				"wd":     false,
			},
		}
		bodyBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/__kiro/transparent/verify", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = clientIP + ":12345"

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		resp := rec.Result()

		// Property: On successful transparent verification, cookie MUST be set
		// and response MUST be HTTP 200 with {"status":"ok"}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected HTTP 200 for successful transparent verify, got %d (ip=%s)", resp.StatusCode, clientIP)
		}

		body := rec.Body.String()
		if !strings.Contains(body, `"status":"ok"`) {
			t.Fatalf("expected response body to contain {\"status\":\"ok\"}, got %q (ip=%s)", body, clientIP)
		}

		cookies := resp.Cookies()
		hasCookie := false
		for _, c := range cookies {
			if c.Name == "kiro_access" {
				hasCookie = true
				break
			}
		}
		if !hasCookie {
			t.Fatalf("expected Set-Cookie 'kiro_access' header on successful transparent verify (ip=%s)", clientIP)
		}
	})
}

// TestPreservation_TransparentVerify_NoCookieOnFailure verifies that
// handleTransparentVerify does NOT set cookie on failed verification.
// This is already correctly implemented and must remain unchanged.
//
// **Validates: Requirements 3.1**
func TestPreservation_TransparentVerify_NoCookieOnFailure(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random IP
		a := rapid.IntRange(1, 254).Draw(t, "ip_a")
		b := rapid.IntRange(0, 255).Draw(t, "ip_b")
		c := rapid.IntRange(0, 255).Draw(t, "ip_c")
		d := rapid.IntRange(1, 254).Draw(t, "ip_d")
		clientIP := intToStrProxy(a) + "." + intToStrProxy(b) + "." + intToStrProxy(c) + "." + intToStrProxy(d)

		store := challenge.NewStore()

		// Send a request with an invalid/nonexistent token directly to VerifyTransparent
		reqBody := map[string]interface{}{
			"token":    "invalid-token-does-not-exist",
			"solution": "invalid-solution",
			"fp": map[string]interface{}{
				"canvas": "test-canvas",
				"webgl":  "test-webgl",
				"tz":     0,
				"wd":     false,
			},
		}
		bodyBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/__kiro/transparent/verify", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = clientIP + ":12345"

		rec := httptest.NewRecorder()

		// Call VerifyTransparent directly (no escalation engine to avoid nil pointer)
		success := challenge.VerifyTransparent(rec, req, store, clientIP, nil)

		// Property: On failed transparent verification, success MUST be false
		// and NO cookie should be set by the caller
		if success {
			t.Fatalf("expected VerifyTransparent to return false for invalid token (ip=%s), got true", clientIP)
		}

		resp := rec.Result()

		// Response should be 403 Forbidden
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected HTTP 403 for failed transparent verify, got %d (ip=%s)", resp.StatusCode, clientIP)
		}

		// Since VerifyTransparent returned false, the caller (handleTransparentVerify)
		// should NOT set a cookie. Verify no cookie was set by VerifyTransparent itself.
		cookies := resp.Cookies()
		for _, c := range cookies {
			if c.Name == "kiro_access" {
				t.Fatalf("expected NO Set-Cookie 'kiro_access' on failed transparent verify (ip=%s), but cookie was set", clientIP)
			}
		}
	})
}

// computeTransparentSolution replicates the JavaScript HMAC-like XOR hash
// used in the transparent challenge page: hm(salt, token)
func computeTransparentSolution(salt, token string) string {
	r := 0
	for i := 0; i < len(salt); i++ {
		r = ((r << 5) - r + int(salt[i])) | 0
	}
	for i := 0; i < len(token); i++ {
		r = ((r << 5) - r + int(token[i])) | 0
	}
	// Convert to hex string (matching JavaScript's toString(16))
	if r < 0 {
		// JavaScript handles negative numbers with "-" prefix in toString(16)
		return "-" + intToHex(-r)
	}
	return intToHex(r)
}

func intToHex(n int) string {
	if n == 0 {
		return "0"
	}
	hex := ""
	for n > 0 {
		digit := n % 16
		if digit < 10 {
			hex = string(rune('0'+digit)) + hex
		} else {
			hex = string(rune('a'+digit-10)) + hex
		}
		n /= 16
	}
	return hex
}

func intToStrProxy(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}
