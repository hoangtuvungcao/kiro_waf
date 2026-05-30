// Feature: waf-system-overhaul, Property 14: Automation User-Agent Detection
// **Validates: Requirements 6.6**
//
// For any User-Agent string chứa các pattern đã biết (chuỗi rỗng, "curl/", "python-requests",
// "sqlmap", "httpclient", "libwww-perl"), hàm `automationUserAgent` SHALL trả về true.
// For any User-Agent string chứa browser identifier hợp lệ (Mozilla/5.0 với Chrome/Firefox/Safari/Edge),
// hàm SHALL trả về false.
package property

import (
	"fmt"
	"testing"

	"kiro_waf/internal/client/ua"

	"pgregory.net/rapid"
)

// knownAttackPatterns are substrings that should trigger automation detection.
var knownAttackPatterns = []string{
	"sqlmap",
	"python-requests",
	"python-urllib",
	"libwww-perl",
	"httpclient",
	"go-http-client",
	"nikto",
	"masscan",
}

// knownPrefixPatterns are prefixes that should trigger automation detection.
var knownPrefixPatterns = []string{
	"curl/",
	"wget/",
}

// TestUADetection_PositiveCase verifies that for any User-Agent string containing
// known attack patterns (empty string, "curl/", "python-requests", "sqlmap",
// "httpclient", "libwww-perl"), IsAutomationUA returns true.
func TestUADetection_PositiveCase(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Strategy: randomly pick between empty UA, substring pattern, or prefix pattern
		strategy := rapid.IntRange(0, 2).Draw(t, "strategy")

		var generatedUA string

		switch strategy {
		case 0:
			// Empty User-Agent — always detected as automation
			generatedUA = ""

		case 1:
			// Embed a known attack substring pattern into a random string
			patternIdx := rapid.IntRange(0, len(knownAttackPatterns)-1).Draw(t, "patternIdx")
			pattern := knownAttackPatterns[patternIdx]

			// Generate random prefix and suffix to embed the pattern within
			prefix := rapid.StringMatching(`[a-zA-Z0-9 /\.\-]{0,30}`).Draw(t, "prefix")
			suffix := rapid.StringMatching(`[a-zA-Z0-9 /\.\-]{0,30}`).Draw(t, "suffix")
			generatedUA = prefix + pattern + suffix

		case 2:
			// Start with a known prefix pattern
			prefixIdx := rapid.IntRange(0, len(knownPrefixPatterns)-1).Draw(t, "prefixIdx")
			prefix := knownPrefixPatterns[prefixIdx]

			// Append random version-like suffix
			version := rapid.StringMatching(`[0-9]{1,3}\.[0-9]{1,3}(\.[0-9]{1,3})?`).Draw(t, "version")
			generatedUA = prefix + version
		}

		// Property: IsAutomationUA MUST return true for UAs with known attack patterns
		if !ua.IsAutomationUA(generatedUA) {
			t.Fatalf("IsAutomationUA returned false for UA with known attack pattern: %q", generatedUA)
		}
	})
}

// TestUADetection_NegativeCase verifies that for any User-Agent string containing
// a valid browser identifier (Mozilla/5.0 with Chrome/Firefox/Safari/Edge),
// IsAutomationUA returns false.
func TestUADetection_NegativeCase(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a valid browser-like UA string with random version numbers
		browserType := rapid.IntRange(0, 3).Draw(t, "browserType")

		majorVersion := rapid.IntRange(80, 130).Draw(t, "majorVersion")
		minorVersion := rapid.IntRange(0, 99).Draw(t, "minorVersion")
		patchVersion := rapid.IntRange(0, 9999).Draw(t, "patchVersion")

		var generatedUA string

		switch browserType {
		case 0:
			// Chrome on Windows
			generatedUA = fmt.Sprintf(
				"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%d.%d.%d.0 Safari/537.36",
				majorVersion, minorVersion, patchVersion,
			)
		case 1:
			// Firefox on Linux
			generatedUA = fmt.Sprintf(
				"Mozilla/5.0 (X11; Linux x86_64; rv:%d.0) Gecko/20100101 Firefox/%d.0",
				majorVersion, majorVersion,
			)
		case 2:
			// Safari on macOS
			generatedUA = fmt.Sprintf(
				"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_%d) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/%d.%d Safari/605.1.15",
				minorVersion%10, majorVersion%20+10, minorVersion%10,
			)
		case 3:
			// Edge on Windows
			generatedUA = fmt.Sprintf(
				"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%d.%d.%d.0 Safari/537.36 Edg/%d.%d.%d.0",
				majorVersion, minorVersion, patchVersion,
				majorVersion, minorVersion, patchVersion,
			)
		}

		// Property: IsAutomationUA MUST return false for valid browser UAs
		if ua.IsAutomationUA(generatedUA) {
			t.Fatalf("IsAutomationUA returned true for valid browser UA: %q", generatedUA)
		}
	})
}
