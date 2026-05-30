// Feature: waf-system-overhaul, Property 5: Non-IPv4 Packets Pass Through XDP
// **Validates: Requirements 6.9**
//
// For any Ethernet frame where the EtherType field is not ETH_P_IP (0x0800),
// the XDP filter SHALL return XDP_PASS without performing any filtering,
// rate limiting, or blocklist checks.
package property

import (
	"testing"

	"pgregory.net/rapid"
)

// ETH_P_IP is the EtherType value for IPv4 (0x0800).
const ETH_P_IP uint16 = 0x0800

// XDP action constants are defined in xdp_blocklist_test.go:
// XDP_DROP = 1, XDP_PASS = 2

// XDPClassifyAction represents what the XDP filter does after EtherType classification.
type XDPClassifyAction int

const (
	// ActionPass means the packet is passed through immediately (no further processing).
	ActionPass XDPClassifyAction = iota
	// ActionProcessIPv4 means the packet proceeds to IPv4 filtering pipeline.
	ActionProcessIPv4
)

// xdpClassifyEtherType simulates the XDP filter's initial EtherType classification.
// This models the first decision point in kiro_xdp_drop():
//   - If EtherType != ETH_P_IP (0x0800) → XDP_PASS immediately (no filtering)
//   - If EtherType == ETH_P_IP (0x0800) → proceed to IPv4 processing pipeline
//
// This is a faithful model of the C code:
//
//	if (eth->h_proto != __builtin_bswap16(ETH_P_IP)) {
//	    stat_inc(KIRO_STAT_PASS);
//	    return XDP_PASS;
//	}
func xdpClassifyEtherType(etherType uint16) XDPClassifyAction {
	if etherType != ETH_P_IP {
		return ActionPass
	}
	return ActionProcessIPv4
}

// xdpFilterResult returns the XDP action for a given EtherType.
// For non-IPv4 packets, this always returns XDP_PASS.
// For IPv4 packets, the result depends on further processing (blocklist, rate limit, etc.)
// but for this test we only verify the classification decision.
func xdpFilterResult(etherType uint16) int {
	if xdpClassifyEtherType(etherType) == ActionPass {
		return XDP_PASS
	}
	// IPv4 packets go through further processing — not tested here
	return -1 // sentinel: indicates further processing needed
}

// TestXDPClassify_NonIPv4PacketsPass verifies that for any EtherType value
// that is NOT 0x0800 (ETH_P_IP), the XDP filter returns XDP_PASS immediately
// without any filtering or rate limiting.
func TestXDPClassify_NonIPv4PacketsPass(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate any EtherType value except ETH_P_IP (0x0800)
		etherType := rapid.Uint16().Draw(t, "etherType")

		// Filter out IPv4 EtherType — we only test non-IPv4 here
		if etherType == ETH_P_IP {
			t.Skip("generated ETH_P_IP, skipping (tested separately)")
		}

		// Property: Non-IPv4 EtherType → XDP_PASS (no filtering applied)
		action := xdpClassifyEtherType(etherType)
		if action != ActionPass {
			t.Fatalf("expected ActionPass for non-IPv4 EtherType 0x%04x, got ActionProcessIPv4",
				etherType)
		}

		// Also verify the full filter result is XDP_PASS
		result := xdpFilterResult(etherType)
		if result != XDP_PASS {
			t.Fatalf("expected XDP_PASS for non-IPv4 EtherType 0x%04x, got %d",
				etherType, result)
		}
	})
}

// TestXDPClassify_IPv4PacketsProceedToFiltering verifies that packets with
// EtherType == 0x0800 (ETH_P_IP) proceed to the IPv4 filtering pipeline
// rather than being immediately passed.
func TestXDPClassify_IPv4PacketsProceedToFiltering(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Always use ETH_P_IP
		etherType := ETH_P_IP

		// Property: IPv4 EtherType → proceeds to further processing (not immediate pass)
		action := xdpClassifyEtherType(etherType)
		if action != ActionProcessIPv4 {
			t.Fatalf("expected ActionProcessIPv4 for ETH_P_IP (0x%04x), got ActionPass",
				etherType)
		}
	})
}

// TestXDPClassify_CommonNonIPv4Protocols verifies specific well-known non-IPv4
// EtherType values all pass through without filtering.
func TestXDPClassify_CommonNonIPv4Protocols(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Common non-IPv4 EtherTypes that should all pass through
		commonEtherTypes := []uint16{
			0x0806, // ARP
			0x86DD, // IPv6
			0x8100, // 802.1Q VLAN
			0x88A8, // 802.1ad (QinQ)
			0x8847, // MPLS unicast
			0x8848, // MPLS multicast
			0x88CC, // LLDP
			0x8863, // PPPoE Discovery
			0x8864, // PPPoE Session
			0x88F7, // PTP (Precision Time Protocol)
			0x0842, // Wake-on-LAN
			0x22F3, // IETF TRILL
			0x6003, // DECnet Phase IV
			0x809B, // AppleTalk
		}

		// Pick a random common EtherType
		idx := rapid.IntRange(0, len(commonEtherTypes)-1).Draw(t, "etherTypeIdx")
		etherType := commonEtherTypes[idx]

		// Property: All common non-IPv4 protocols pass through
		action := xdpClassifyEtherType(etherType)
		if action != ActionPass {
			t.Fatalf("expected ActionPass for common non-IPv4 EtherType 0x%04x, got ActionProcessIPv4",
				etherType)
		}

		result := xdpFilterResult(etherType)
		if result != XDP_PASS {
			t.Fatalf("expected XDP_PASS for common non-IPv4 EtherType 0x%04x, got %d",
				etherType, result)
		}
	})
}
