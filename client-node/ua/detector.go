package ua

import "strings"

// knownAttackPatterns contains substrings (lowercase) that indicate
// automated attack tools or non-browser HTTP clients.
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

// knownPrefixPatterns contains prefixes (lowercase) that indicate
// automated tools when the UA starts with them.
var knownPrefixPatterns = []string{
	"curl/",
	"wget/",
}

// IsAutomationUA returns true if the given User-Agent string matches
// a known automation tool or attack pattern. Matching is case-insensitive.
//
// Detection rules:
//   - Empty string → true (missing UA is suspicious)
//   - Contains known attack tool substring → true
//   - Starts with known tool prefix → true
//   - Valid browser UAs (Mozilla/5.0 with Chrome/Firefox/Safari/Edge) → false
//   - Custom UAs that don't match attack patterns → false
func IsAutomationUA(ua string) bool {
	// Empty User-Agent is always suspicious
	if ua == "" {
		return true
	}

	lower := strings.ToLower(ua)

	// Check substring patterns
	for _, pattern := range knownAttackPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	// Check prefix patterns
	for _, prefix := range knownPrefixPatterns {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}

	return false
}
