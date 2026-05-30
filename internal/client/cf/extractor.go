// Package cf provides Cloudflare proxy compatibility utilities.
// CFExtractor identifies requests from Cloudflare edge servers and extracts
// the real client IP from the CF-Connecting-IP header when the peer is trusted.
package cf

import (
	"net"
	"net/http"
	"strings"
)

// cloudflareIPv4Ranges contains the known Cloudflare IPv4 CIDR ranges.
// Source: https://www.cloudflare.com/ips-v4/
var cloudflareIPv4Ranges = []string{
	"173.245.48.0/20",
	"103.21.244.0/22",
	"103.22.200.0/22",
	"103.31.4.0/22",
	"141.101.64.0/18",
	"108.162.192.0/18",
	"190.93.240.0/20",
	"188.114.96.0/20",
	"197.234.240.0/22",
	"198.41.128.0/17",
	"162.158.0.0/15",
	"104.16.0.0/13",
	"104.24.0.0/14",
	"172.64.0.0/13",
	"131.0.72.0/22",
}

// TrustModeStrict only trusts CF-Connecting-IP when the peer IP is in Cloudflare ranges.
const TrustModeStrict = "strict"

// TrustModePermissive always trusts the CF-Connecting-IP header regardless of peer IP.
const TrustModePermissive = "permissive"

// CFExtractor extracts the real client IP from requests proxied through Cloudflare.
// In strict mode, it only trusts the CF-Connecting-IP header when the peer IP
// belongs to a known Cloudflare IP range. In permissive mode, it always trusts the header.
type CFExtractor struct {
	trustedRanges []*net.IPNet
	trustMode     string
}

// NewCFExtractor creates a new CFExtractor with the specified trust mode.
// Valid modes are "strict" and "permissive". Any other value defaults to "strict".
func NewCFExtractor(mode string) *CFExtractor {
	if mode != TrustModePermissive {
		mode = TrustModeStrict
	}

	ranges := make([]*net.IPNet, 0, len(cloudflareIPv4Ranges))
	for _, cidr := range cloudflareIPv4Ranges {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			// Should never happen with hardcoded valid CIDRs
			continue
		}
		ranges = append(ranges, ipNet)
	}

	return &CFExtractor{
		trustedRanges: ranges,
		trustMode:     mode,
	}
}

// ExtractClientIP returns the real client IP from the request.
// If the peer IP is a Cloudflare address (or trust mode is permissive) and
// the CF-Connecting-IP header is present, it returns the header value.
// Otherwise, it falls back to extracting the IP from RemoteAddr.
func (e *CFExtractor) ExtractClientIP(r *http.Request) string {
	cfIP := strings.TrimSpace(r.Header.Get("CF-Connecting-IP"))
	if cfIP == "" {
		return extractIPFromAddr(r.RemoteAddr)
	}

	if e.trustMode == TrustModePermissive {
		return cfIP
	}

	// Strict mode: only trust CF-Connecting-IP if peer is a Cloudflare IP
	peerIP := extractIPFromAddr(r.RemoteAddr)
	if e.isInCloudflareRange(peerIP) {
		return cfIP
	}

	// Peer is not Cloudflare — ignore the header, use peer IP
	return peerIP
}

// IsCloudflarePeer checks whether the given remote address belongs to a known
// Cloudflare IP range. The remoteAddr can be in "ip:port" format or a bare IP.
func (e *CFExtractor) IsCloudflarePeer(remoteAddr string) bool {
	ip := extractIPFromAddr(remoteAddr)
	return e.isInCloudflareRange(ip)
}

// isInCloudflareRange checks if the given IP string falls within any trusted Cloudflare range.
func (e *CFExtractor) isInCloudflareRange(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, ipNet := range e.trustedRanges {
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

// extractIPFromAddr extracts the IP portion from an address that may be in "ip:port" format.
func extractIPFromAddr(addr string) string {
	if addr == "" {
		return ""
	}
	// Try to split host:port
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// No port, return as-is (might be a bare IP)
		return addr
	}
	return host
}
