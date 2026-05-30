// Feature: waf-system-overhaul, Property 4: Challenge Page No External Dependencies
// **Validates: Requirements 2.4**
//
// For any challenge page HTML được sinh ra (cả JS PoW và Hold captcha),
// nội dung HTML SHALL không chứa bất kỳ URL nào trỏ đến domain bên ngoài
// (không có `http://` hoặc `https://` references đến host khác ngoài
// endpoint xác minh nội bộ `/__kiro/`).
package property

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"kiro_waf/internal/client/challenge"

	"pgregory.net/rapid"
)

// urlPattern matches http:// and https:// URLs in HTML content.
var urlPattern = regexp.MustCompile(`https?://[^\s"'<>]+`)

// isInternalURL checks if a URL is an internal /__kiro/ reference.
func isInternalURL(url string) bool {
	// Internal references use relative paths like "/__kiro/challenge/verify"
	// They should not appear as full http(s):// URLs in the HTML.
	// Any http:// or https:// URL is considered external unless it contains /__kiro/ path.
	return false
}

// checkNoExternalURLs scans HTML content for external URLs.
// Returns a list of external URLs found (empty if none).
func checkNoExternalURLs(html string) []string {
	matches := urlPattern.FindAllString(html, -1)
	var external []string
	for _, u := range matches {
		// Allow internal /__kiro/ references if they somehow appear as full URLs
		// (e.g., in a comment or documentation). In practice, the templates use
		// relative paths, so no http(s):// URLs should reference /__kiro/.
		if !isInternalURL(u) {
			external = append(external, u)
		}
	}
	return external
}

// TestChallengePageNoExternalDeps_PoW verifies that the JS PoW challenge page
// does not contain any external URL references (http:// or https://).
func TestChallengePageNoExternalDeps_PoW(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random client IP
		ip := fmt.Sprintf("%d.%d.%d.%d",
			rapid.IntRange(1, 254).Draw(t, "ip_octet1"),
			rapid.IntRange(0, 255).Draw(t, "ip_octet2"),
			rapid.IntRange(0, 255).Draw(t, "ip_octet3"),
			rapid.IntRange(1, 254).Draw(t, "ip_octet4"),
		)

		// Generate random difficulty level (1-6)
		difficulty := rapid.IntRange(1, 6).Draw(t, "difficulty")

		// Create a challenge store
		store := challenge.NewStore()

		// Create an HTTP request to simulate the challenge page request
		req := httptest.NewRequest(http.MethodGet, "/__kiro/challenge", nil)
		rec := httptest.NewRecorder()

		// Render the PoW challenge page
		challenge.ServeChallengePage(rec, req, store, difficulty, 90*time.Second, ip)

		// Get the rendered HTML
		html := rec.Body.String()

		if html == "" {
			t.Fatal("ServeChallengePage produced empty HTML")
		}

		// Property: No external URLs in the HTML
		externalURLs := checkNoExternalURLs(html)
		if len(externalURLs) > 0 {
			t.Fatalf("PoW challenge page contains external URLs (clientIP=%s, difficulty=%d): %v",
				ip, difficulty, externalURLs)
		}
	})
}

// TestChallengePageNoExternalDeps_Hold verifies that the Hold captcha page
// does not contain any external URL references (http:// or https://).
func TestChallengePageNoExternalDeps_Hold(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random client IP
		ip := fmt.Sprintf("%d.%d.%d.%d",
			rapid.IntRange(1, 254).Draw(t, "ip_octet1"),
			rapid.IntRange(0, 255).Draw(t, "ip_octet2"),
			rapid.IntRange(0, 255).Draw(t, "ip_octet3"),
			rapid.IntRange(1, 254).Draw(t, "ip_octet4"),
		)

		// Generate random hold seconds (1-10)
		holdSeconds := rapid.IntRange(1, 10).Draw(t, "holdSeconds")

		// Create a challenge store
		store := challenge.NewStore()

		// Create an HTTP request to simulate the hold page request
		req := httptest.NewRequest(http.MethodGet, "/__kiro/hold", nil)
		rec := httptest.NewRecorder()

		// Render the Hold captcha page
		challenge.ServeHoldPage(rec, req, store, holdSeconds, 90*time.Second, ip)

		// Get the rendered HTML
		html := rec.Body.String()

		if html == "" {
			t.Fatal("ServeHoldPage produced empty HTML")
		}

		// Property: No external URLs in the HTML
		externalURLs := checkNoExternalURLs(html)
		if len(externalURLs) > 0 {
			t.Fatalf("Hold captcha page contains external URLs (clientIP=%s, holdSeconds=%d): %v",
				ip, holdSeconds, externalURLs)
		}
	})
}
