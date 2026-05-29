// Feature: waf-system-overhaul, Property 15: Private Source IP Detection
// **Validates: Requirements 7.3**
//
// For any IPv4 address thuộc dải RFC 1918 (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16),
// loopback (127.0.0.0/8), hoặc link-local (169.254.0.0/16), hàm `private_source_v4`
// SHALL trả về 1. For any IPv4 address thuộc dải public routable, hàm SHALL trả về 0.
package property

import (
	"encoding/binary"
	"fmt"
	"testing"

	"pgregory.net/rapid"
)

// privateSourceV4 is a Go simulation of the private_source_v4 function from xdp_filter.c.
// It takes an IPv4 address in network byte order (big-endian) and returns true if the
// address belongs to a private/reserved range.
func privateSourceV4(networkOrderAddr uint32) bool {
	// Convert from network byte order (big-endian) to host byte order (little-endian)
	// This mirrors __builtin_bswap32(network_order_saddr) in the C code.
	ip := bswap32(networkOrderAddr)

	// 10.0.0.0/8
	if (ip & 0xff000000) == 0x0a000000 {
		return true
	}
	// 172.16.0.0/12
	if (ip & 0xfff00000) == 0xac100000 {
		return true
	}
	// 192.168.0.0/16
	if (ip & 0xffff0000) == 0xc0a80000 {
		return true
	}
	// 127.0.0.0/8 (loopback)
	if (ip & 0xff000000) == 0x7f000000 {
		return true
	}
	// 169.254.0.0/16 (link-local)
	if (ip & 0xffff0000) == 0xa9fe0000 {
		return true
	}
	return false
}

// bswap32 reverses the byte order of a 32-bit integer (network ↔ host byte order).
func bswap32(v uint32) uint32 {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return binary.LittleEndian.Uint32(b)
}

// ipToNetworkOrder converts a host-order IPv4 address (e.g., 0x0A010203 for 10.1.2.3)
// to network byte order (big-endian) as it would appear in a packet's iphdr.saddr.
func ipToNetworkOrder(hostOrder uint32) uint32 {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, hostOrder)
	return binary.LittleEndian.Uint32(b)
}

// formatIP converts a host-order IPv4 address to dotted-decimal string for debugging.
func formatIP(hostOrder uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d",
		(hostOrder>>24)&0xff,
		(hostOrder>>16)&0xff,
		(hostOrder>>8)&0xff,
		hostOrder&0xff)
}

// --- Generators for private IP ranges ---

// gen10Net generates a random IP in 10.0.0.0/8 (host order).
func gen10Net(t *rapid.T) uint32 {
	// 10.x.x.x → first octet = 0x0a, rest random
	rest := rapid.Uint32Range(0, 0x00ffffff).Draw(t, "10net_rest")
	return 0x0a000000 | rest
}

// gen172Net generates a random IP in 172.16.0.0/12 (host order).
func gen172Net(t *rapid.T) uint32 {
	// 172.16.0.0 - 172.31.255.255
	// First 12 bits = 0xac1 (172.16-31), remaining 20 bits random
	rest := rapid.Uint32Range(0, 0x000fffff).Draw(t, "172net_rest")
	return 0xac100000 | rest
}

// gen192Net generates a random IP in 192.168.0.0/16 (host order).
func gen192Net(t *rapid.T) uint32 {
	// 192.168.x.x → first two octets = 0xc0a8, rest random
	rest := rapid.Uint32Range(0, 0x0000ffff).Draw(t, "192net_rest")
	return 0xc0a80000 | rest
}

// genLoopback generates a random IP in 127.0.0.0/8 (host order).
func genLoopback(t *rapid.T) uint32 {
	// 127.x.x.x → first octet = 0x7f, rest random
	rest := rapid.Uint32Range(0, 0x00ffffff).Draw(t, "loopback_rest")
	return 0x7f000000 | rest
}

// genLinkLocal generates a random IP in 169.254.0.0/16 (host order).
func genLinkLocal(t *rapid.T) uint32 {
	// 169.254.x.x → first two octets = 0xa9fe, rest random
	rest := rapid.Uint32Range(0, 0x0000ffff).Draw(t, "linklocal_rest")
	return 0xa9fe0000 | rest
}

// isPrivateHostOrder checks if a host-order IP falls in any private range.
// Used by the public IP generator to filter out private addresses.
func isPrivateHostOrder(ip uint32) bool {
	if (ip & 0xff000000) == 0x0a000000 {
		return true
	}
	if (ip & 0xfff00000) == 0xac100000 {
		return true
	}
	if (ip & 0xffff0000) == 0xc0a80000 {
		return true
	}
	if (ip & 0xff000000) == 0x7f000000 {
		return true
	}
	if (ip & 0xffff0000) == 0xa9fe0000 {
		return true
	}
	return false
}

// genPublicIP generates a random public routable IPv4 address (host order)
// that does NOT fall in any private/reserved range.
func genPublicIP(t *rapid.T) uint32 {
	// Generate IPs from known public ranges to avoid rejection sampling.
	// Public ranges used: 1.0.0.0/8, 8.0.0.0/8, 44.0.0.0/8, 100.64.0.0-100.127.x.x,
	// 198.51.100.0/24, 203.0.113.0/24, etc.
	// We use a simple approach: pick a first octet that's clearly public.
	publicFirstOctets := []uint32{
		1, 2, 3, 4, 5, 6, 7, 8, 9,
		11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
		21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
		33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48,
		49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63, 64,
		65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79, 80,
		81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 96,
		97, 98, 99, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110,
		111, 112, 113, 114, 115, 116, 117, 118, 119, 120, 121, 122, 123,
		124, 125, 126,
		// Skip 127 (loopback)
		128, 129, 130, 131, 132, 133, 134, 135, 136, 137, 138, 139, 140,
		141, 142, 143, 144, 145, 146, 147, 148, 149, 150, 151, 152, 153,
		154, 155, 156, 157, 158, 159, 160, 161, 162, 163, 164, 165, 166,
		167, 168,
		// Skip 169 partially (169.254.x.x is link-local, but 169.0-253.x.x is fine)
		170, 171,
		// Skip 172.16-31 (private), but 172.0-15 and 172.32-255 are public
		173, 174, 175, 176, 177, 178, 179, 180, 181, 182, 183, 184, 185,
		186, 187, 188, 189, 190, 191,
		// Skip 192.168 (private), but 192.0-167 and 192.169-255 are fine
		193, 194, 195, 196, 197, 198, 199, 200, 201, 202, 203, 204, 205,
		206, 207, 208, 209, 210, 211, 212, 213, 214, 215, 216, 217, 218,
		219, 220, 221, 222, 223,
	}

	idx := rapid.IntRange(0, len(publicFirstOctets)-1).Draw(t, "public_octet_idx")
	firstOctet := publicFirstOctets[idx]
	rest := rapid.Uint32Range(0, 0x00ffffff).Draw(t, "public_rest")
	ip := (firstOctet << 24) | rest

	// Double-check: if by chance we hit a private range (e.g., 172.16-31.x.x from octet 172
	// is excluded from the list, but 169.254.x.x could slip through if 169 were included),
	// we filter it out. Since we excluded problematic first octets, this should rarely trigger.
	if isPrivateHostOrder(ip) {
		// Fallback to a known public IP with random last octets
		rest2 := rapid.Uint32Range(0, 0x0000ffff).Draw(t, "fallback_rest")
		ip = (8 << 24) | (8 << 16) | rest2 // 8.8.x.x (Google DNS range, definitely public)
	}

	return ip
}

// TestPrivateIP_PrivateRangesDetected verifies that for any IPv4 address in a private range,
// privateSourceV4 returns true.
func TestPrivateIP_PrivateRangesDetected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Pick a random private range and generate an IP within it
		rangeIdx := rapid.IntRange(0, 4).Draw(t, "range")
		var hostOrderIP uint32
		var rangeName string

		switch rangeIdx {
		case 0:
			hostOrderIP = gen10Net(t)
			rangeName = "10.0.0.0/8"
		case 1:
			hostOrderIP = gen172Net(t)
			rangeName = "172.16.0.0/12"
		case 2:
			hostOrderIP = gen192Net(t)
			rangeName = "192.168.0.0/16"
		case 3:
			hostOrderIP = genLoopback(t)
			rangeName = "127.0.0.0/8"
		case 4:
			hostOrderIP = genLinkLocal(t)
			rangeName = "169.254.0.0/16"
		}

		// Convert to network byte order (as it would appear in iphdr.saddr)
		networkOrder := ipToNetworkOrder(hostOrderIP)

		// Property: privateSourceV4 MUST return true for private IPs
		if !privateSourceV4(networkOrder) {
			t.Fatalf("privateSourceV4 returned false for private IP %s (range %s, host=0x%08x, net=0x%08x)",
				formatIP(hostOrderIP), rangeName, hostOrderIP, networkOrder)
		}
	})
}

// TestPrivateIP_PublicRangesNotDetected verifies that for any IPv4 address in a public
// routable range, privateSourceV4 returns false.
func TestPrivateIP_PublicRangesNotDetected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		hostOrderIP := genPublicIP(t)

		// Convert to network byte order
		networkOrder := ipToNetworkOrder(hostOrderIP)

		// Property: privateSourceV4 MUST return false for public IPs
		if privateSourceV4(networkOrder) {
			t.Fatalf("privateSourceV4 returned true for public IP %s (host=0x%08x, net=0x%08x)",
				formatIP(hostOrderIP), hostOrderIP, networkOrder)
		}
	})
}
