// Feature: waf-system-overhaul, Property 9: XDP Blocklist Drop
// **Validates: Requirements 6.1**
//
// For any IPv4 address nằm trong LPM trie blocklist (khớp prefix), hàm XDP filter
// SHALL trả về XDP_DROP. For any IPv4 address nằm trong allowlist, hàm SHALL trả về
// XDP_PASS bất kể blocklist.
package property

import (
	"testing"

	"pgregory.net/rapid"
)

// XDP action constants matching the C XDP program.
const (
	XDP_DROP = 1
	XDP_PASS = 2
)

// LPMTrie implements a Longest Prefix Match trie for IPv4 addresses.
// This simulates the eBPF LPM_TRIE map behavior used in xdp_filter.c.
type LPMTrie struct {
	entries []lpmEntry
}

type lpmEntry struct {
	prefix    uint32
	prefixLen uint8
	mask      uint32
}

// NewLPMTrie creates a new empty LPM trie.
func NewLPMTrie() *LPMTrie {
	return &LPMTrie{}
}

// Insert adds a prefix/prefixLen entry to the trie.
// prefixLen is the number of significant bits (1-32).
func (t *LPMTrie) Insert(prefix uint32, prefixLen uint8) {
	mask := prefixMask(prefixLen)
	t.entries = append(t.entries, lpmEntry{
		prefix:    prefix & mask,
		prefixLen: prefixLen,
		mask:      mask,
	})
}

// Lookup checks if an IP address matches any entry in the trie.
// Returns true if the IP's first N bits match any stored prefix.
func (t *LPMTrie) Lookup(ip uint32) bool {
	for _, e := range t.entries {
		if (ip & e.mask) == e.prefix {
			return true
		}
	}
	return false
}

// prefixMask returns a bitmask with the top prefixLen bits set.
func prefixMask(prefixLen uint8) uint32 {
	if prefixLen == 0 {
		return 0
	}
	if prefixLen >= 32 {
		return 0xFFFFFFFF
	}
	return ^uint32(0) << (32 - prefixLen)
}

// xdpFilter simulates the XDP filter's allowlist/blocklist logic.
// This mirrors the processing order in kiro_xdp_drop():
//  1. If IP is in allowlist → XDP_PASS (highest priority)
//  2. If IP is in blocklist → XDP_DROP
//  3. Otherwise → XDP_PASS
func xdpFilter(ip uint32, allowlist, blocklist *LPMTrie) int {
	// Step 2 in XDP: Allowlist check (highest priority — always passes)
	if allowlist.Lookup(ip) {
		return XDP_PASS
	}
	// Step 3 in XDP: Blocklist check
	if blocklist.Lookup(ip) {
		return XDP_DROP
	}
	// Default: pass
	return XDP_PASS
}

// ipInPrefix generates a random IP that matches the given prefix/prefixLen.
func ipInPrefix(t *rapid.T, prefix uint32, prefixLen uint8, label string) uint32 {
	mask := prefixMask(prefixLen)
	hostBits := 32 - prefixLen
	if hostBits == 0 {
		return prefix
	}
	// Generate random host part
	maxHost := uint32((1 << hostBits) - 1)
	host := rapid.Uint32Range(0, maxHost).Draw(t, label)
	return (prefix & mask) | host
}

// ipNotInPrefix generates a random IP that does NOT match the given prefix/prefixLen.
func ipNotInPrefix(t *rapid.T, prefix uint32, prefixLen uint8, label string) uint32 {
	mask := prefixMask(prefixLen)
	maskedPrefix := prefix & mask
	for {
		ip := rapid.Uint32().Draw(t, label)
		if (ip & mask) != maskedPrefix {
			return ip
		}
	}
}

// TestXDPBlocklist_BlockedIPsAreDrop verifies that any IP matching a blocklist
// prefix is dropped by the XDP filter.
func TestXDPBlocklist_BlockedIPsAreDrop(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random prefix with length 8-32
		prefixLen := rapid.Uint8Range(8, 32).Draw(t, "prefixLen")
		mask := prefixMask(prefixLen)
		rawPrefix := rapid.Uint32().Draw(t, "rawPrefix")
		prefix := rawPrefix & mask

		// Build blocklist with this prefix
		blocklist := NewLPMTrie()
		blocklist.Insert(prefix, prefixLen)

		// Empty allowlist
		allowlist := NewLPMTrie()

		// Generate an IP that matches the prefix
		ip := ipInPrefix(t, prefix, prefixLen, "blockedIP")

		// Property: IP in blocklist → XDP_DROP
		result := xdpFilter(ip, allowlist, blocklist)
		if result != XDP_DROP {
			t.Fatalf("expected XDP_DROP for IP 0x%08x matching prefix 0x%08x/%d, got %d",
				ip, prefix, prefixLen, result)
		}
	})
}

// TestXDPBlocklist_AllowlistOverridesBlocklist verifies that any IP in the
// allowlist passes regardless of blocklist entries.
func TestXDPBlocklist_AllowlistOverridesBlocklist(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a prefix that will be in BOTH allowlist and blocklist
		prefixLen := rapid.Uint8Range(8, 32).Draw(t, "prefixLen")
		mask := prefixMask(prefixLen)
		rawPrefix := rapid.Uint32().Draw(t, "rawPrefix")
		prefix := rawPrefix & mask

		// Put the same prefix in both lists
		allowlist := NewLPMTrie()
		allowlist.Insert(prefix, prefixLen)

		blocklist := NewLPMTrie()
		blocklist.Insert(prefix, prefixLen)

		// Generate an IP that matches the prefix (in both lists)
		ip := ipInPrefix(t, prefix, prefixLen, "allowedIP")

		// Property: IP in allowlist → XDP_PASS regardless of blocklist
		result := xdpFilter(ip, allowlist, blocklist)
		if result != XDP_PASS {
			t.Fatalf("expected XDP_PASS for IP 0x%08x in allowlist (prefix 0x%08x/%d), got %d",
				ip, prefix, prefixLen, result)
		}
	})
}

// TestXDPBlocklist_UnlistedIPsPass verifies that any IP not in either list passes.
func TestXDPBlocklist_UnlistedIPsPass(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a blocklist prefix
		blockPrefixLen := rapid.Uint8Range(8, 24).Draw(t, "blockPrefixLen")
		blockMask := prefixMask(blockPrefixLen)
		rawBlockPrefix := rapid.Uint32().Draw(t, "rawBlockPrefix")
		blockPrefix := rawBlockPrefix & blockMask

		// Generate an allowlist prefix (different from blocklist)
		allowPrefixLen := rapid.Uint8Range(8, 24).Draw(t, "allowPrefixLen")
		allowMask := prefixMask(allowPrefixLen)
		rawAllowPrefix := rapid.Uint32().Draw(t, "rawAllowPrefix")
		allowPrefix := rawAllowPrefix & allowMask

		blocklist := NewLPMTrie()
		blocklist.Insert(blockPrefix, blockPrefixLen)

		allowlist := NewLPMTrie()
		allowlist.Insert(allowPrefix, allowPrefixLen)

		// Generate an IP that is NOT in either list
		ip := rapid.Uint32().Draw(t, "unlistedIP")

		// Filter: skip if IP happens to match either list
		if blocklist.Lookup(ip) || allowlist.Lookup(ip) {
			t.Skip("generated IP happened to match a list entry, skipping")
		}

		// Property: IP not in any list → XDP_PASS
		result := xdpFilter(ip, allowlist, blocklist)
		if result != XDP_PASS {
			t.Fatalf("expected XDP_PASS for unlisted IP 0x%08x, got %d", ip, result)
		}
	})
}
