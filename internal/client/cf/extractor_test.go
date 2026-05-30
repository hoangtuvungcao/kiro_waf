package cf

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewCFExtractor_DefaultsToStrict(t *testing.T) {
	e := NewCFExtractor("")
	if e.trustMode != TrustModeStrict {
		t.Errorf("expected strict mode, got %q", e.trustMode)
	}
}

func TestNewCFExtractor_PermissiveMode(t *testing.T) {
	e := NewCFExtractor("permissive")
	if e.trustMode != TrustModePermissive {
		t.Errorf("expected permissive mode, got %q", e.trustMode)
	}
}

func TestNewCFExtractor_InvalidModeDefaultsToStrict(t *testing.T) {
	e := NewCFExtractor("invalid")
	if e.trustMode != TrustModeStrict {
		t.Errorf("expected strict mode for invalid input, got %q", e.trustMode)
	}
}

func TestNewCFExtractor_LoadsRanges(t *testing.T) {
	e := NewCFExtractor("strict")
	if len(e.trustedRanges) != len(cloudflareIPv4Ranges) {
		t.Errorf("expected %d ranges, got %d", len(cloudflareIPv4Ranges), len(e.trustedRanges))
	}
}

func TestIsCloudflarePeer_KnownCFIP(t *testing.T) {
	e := NewCFExtractor("strict")

	// 173.245.48.1 is within 173.245.48.0/20
	if !e.IsCloudflarePeer("173.245.48.1:443") {
		t.Error("expected 173.245.48.1 to be recognized as Cloudflare peer")
	}
}

func TestIsCloudflarePeer_KnownCFIP_BareIP(t *testing.T) {
	e := NewCFExtractor("strict")

	if !e.IsCloudflarePeer("104.16.0.1") {
		t.Error("expected 104.16.0.1 to be recognized as Cloudflare peer")
	}
}

func TestIsCloudflarePeer_NonCFIP(t *testing.T) {
	e := NewCFExtractor("strict")

	if e.IsCloudflarePeer("8.8.8.8:443") {
		t.Error("expected 8.8.8.8 to NOT be recognized as Cloudflare peer")
	}
}

func TestIsCloudflarePeer_InvalidIP(t *testing.T) {
	e := NewCFExtractor("strict")

	if e.IsCloudflarePeer("not-an-ip") {
		t.Error("expected invalid IP to return false")
	}
}

func TestIsCloudflarePeer_EmptyAddr(t *testing.T) {
	e := NewCFExtractor("strict")

	if e.IsCloudflarePeer("") {
		t.Error("expected empty address to return false")
	}
}

func TestExtractClientIP_StrictMode_CFPeer_WithHeader(t *testing.T) {
	e := NewCFExtractor("strict")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "173.245.48.1:12345" // Cloudflare IP
	req.Header.Set("CF-Connecting-IP", "203.0.113.50")

	ip := e.ExtractClientIP(req)
	if ip != "203.0.113.50" {
		t.Errorf("expected 203.0.113.50, got %q", ip)
	}
}

func TestExtractClientIP_StrictMode_NonCFPeer_WithHeader(t *testing.T) {
	e := NewCFExtractor("strict")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "8.8.8.8:12345" // Not a Cloudflare IP
	req.Header.Set("CF-Connecting-IP", "203.0.113.50")

	ip := e.ExtractClientIP(req)
	if ip != "8.8.8.8" {
		t.Errorf("expected 8.8.8.8 (peer IP, header ignored), got %q", ip)
	}
}

func TestExtractClientIP_StrictMode_NoCFHeader(t *testing.T) {
	e := NewCFExtractor("strict")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "173.245.48.1:12345"

	ip := e.ExtractClientIP(req)
	if ip != "173.245.48.1" {
		t.Errorf("expected 173.245.48.1 (RemoteAddr), got %q", ip)
	}
}

func TestExtractClientIP_PermissiveMode_NonCFPeer_WithHeader(t *testing.T) {
	e := NewCFExtractor("permissive")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "8.8.8.8:12345" // Not a Cloudflare IP
	req.Header.Set("CF-Connecting-IP", "203.0.113.50")

	ip := e.ExtractClientIP(req)
	if ip != "203.0.113.50" {
		t.Errorf("expected 203.0.113.50 (permissive trusts header), got %q", ip)
	}
}

func TestExtractClientIP_PermissiveMode_NoHeader(t *testing.T) {
	e := NewCFExtractor("permissive")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "8.8.8.8:12345"

	ip := e.ExtractClientIP(req)
	if ip != "8.8.8.8" {
		t.Errorf("expected 8.8.8.8 (no header, fallback to RemoteAddr), got %q", ip)
	}
}

func TestExtractClientIP_WhitespaceInHeader(t *testing.T) {
	e := NewCFExtractor("permissive")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "173.245.48.1:12345"
	req.Header.Set("CF-Connecting-IP", "  203.0.113.50  ")

	ip := e.ExtractClientIP(req)
	if ip != "203.0.113.50" {
		t.Errorf("expected trimmed IP 203.0.113.50, got %q", ip)
	}
}

func TestExtractIPFromAddr_HostPort(t *testing.T) {
	result := extractIPFromAddr("192.168.1.1:8080")
	if result != "192.168.1.1" {
		t.Errorf("expected 192.168.1.1, got %q", result)
	}
}

func TestExtractIPFromAddr_BareIP(t *testing.T) {
	result := extractIPFromAddr("192.168.1.1")
	if result != "192.168.1.1" {
		t.Errorf("expected 192.168.1.1, got %q", result)
	}
}

func TestExtractIPFromAddr_Empty(t *testing.T) {
	result := extractIPFromAddr("")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

// Test various Cloudflare IP ranges to ensure all are loaded correctly.
func TestIsCloudflarePeer_AllRanges(t *testing.T) {
	e := NewCFExtractor("strict")

	// One IP from each known range
	testCases := []struct {
		ip       string
		expected bool
	}{
		{"173.245.48.1", true},   // 173.245.48.0/20
		{"103.21.244.1", true},   // 103.21.244.0/22
		{"103.22.200.1", true},   // 103.22.200.0/22
		{"103.31.4.1", true},     // 103.31.4.0/22
		{"141.101.64.1", true},   // 141.101.64.0/18
		{"108.162.192.1", true},  // 108.162.192.0/18
		{"190.93.240.1", true},   // 190.93.240.0/20
		{"188.114.96.1", true},   // 188.114.96.0/20
		{"197.234.240.1", true},  // 197.234.240.0/22
		{"198.41.128.1", true},   // 198.41.128.0/17
		{"162.158.0.1", true},    // 162.158.0.0/15
		{"104.16.0.1", true},     // 104.16.0.0/13
		{"104.24.0.1", true},     // 104.24.0.0/14
		{"172.64.0.1", true},     // 172.64.0.0/13
		{"131.0.72.1", true},     // 131.0.72.0/22
		{"1.1.1.1", false},       // Not Cloudflare proxy range
		{"192.168.1.1", false},   // Private IP
		{"10.0.0.1", false},      // Private IP
	}

	for _, tc := range testCases {
		got := e.IsCloudflarePeer(tc.ip)
		if got != tc.expected {
			t.Errorf("IsCloudflarePeer(%q) = %v, want %v", tc.ip, got, tc.expected)
		}
	}
}
