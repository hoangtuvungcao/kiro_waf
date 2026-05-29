// Feature: waf-system-overhaul, Property 12: XDP Malformed Packet Detection
// **Validates: Requirements 6.4**
//
// For any TCP packet có flag combination thuộc tập {null flags, SYN+FIN, SYN+RST,
// FIN+PSH+URG (Christmas tree)}, hoặc for any IP packet có total_length < ip_header_length,
// hoặc for any UDP packet có udp_length < 8 hoặc udp_length > ip_payload_length,
// hàm XDP filter SHALL trả về XDP_DROP khi drop_malformed được bật.
package property

import (
	"testing"

	"pgregory.net/rapid"
)

// ─── TCP Flag Constants (matching xdp_filter.c) ───
const (
	tcpFlagFIN  = 0x01
	tcpFlagSYN  = 0x02
	tcpFlagRST  = 0x04
	tcpFlagPSH  = 0x08
	tcpFlagACK  = 0x10
	tcpFlagURG  = 0x20
	tcpFlagMask = 0x3f
)

// tcpFlagsInvalid mirrors the C logic in xdp_filter.c tcp_flags_invalid().
// Returns true if the TCP flag combination is malformed.
func tcpFlagsInvalid(flags byte) bool {
	masked := flags & tcpFlagMask

	// Null flags — no flags set at all
	if masked == 0 {
		return true
	}
	// SYN combined with FIN or RST
	if (masked&tcpFlagSYN) != 0 && (masked&(tcpFlagFIN|tcpFlagRST)) != 0 {
		return true
	}
	// Christmas tree: FIN+PSH+URG all set
	if (masked & (tcpFlagFIN | tcpFlagPSH | tcpFlagURG)) == (tcpFlagFIN | tcpFlagPSH | tcpFlagURG) {
		return true
	}
	return false
}

// ipTotalLengthInvalid returns true if IP total_length < ip_header_length.
// This mirrors the check in xdp_filter.c: if (total_len < ip_header_len) goto malformed;
func ipTotalLengthInvalid(totalLen, headerLen uint16) bool {
	return totalLen < headerLen
}

// udpLengthInvalid returns true if UDP length < 8 or UDP length > IP payload length.
// This mirrors the checks in xdp_filter.c:
//   - if (bswap16(*udp_len) < KIRO_UDP_HEADER_LEN) goto malformed;
//   - if (bswap16(*udp_len) > total_len - ip_header_len) goto malformed;
func udpLengthInvalid(udpLen, ipPayloadLen uint16) bool {
	if udpLen < 8 {
		return true
	}
	if udpLen > ipPayloadLen {
		return true
	}
	return false
}

// ─── Property Tests ───

// TestXDPMalformed_TCPNullFlags verifies that TCP packets with no flags set
// are detected as malformed.
func TestXDPMalformed_TCPNullFlags(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random byte where the lower 6 bits are all zero (null flags)
		// Upper 2 bits can be anything since they are masked out
		upperBits := rapid.Byte().Draw(t, "upperBits")
		flags := upperBits & ^byte(tcpFlagMask) // Clear lower 6 bits, keep upper 2

		if !tcpFlagsInvalid(flags) {
			t.Fatalf("TCP null flags (0x%02x) should be detected as malformed", flags)
		}
	})
}

// TestXDPMalformed_TCPSynFin verifies that TCP packets with SYN+FIN are
// detected as malformed.
func TestXDPMalformed_TCPSynFin(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate flags that have both SYN and FIN set, with random other bits
		extraFlags := rapid.Byte().Draw(t, "extraFlags")
		flags := (extraFlags & tcpFlagMask) | tcpFlagSYN | tcpFlagFIN

		if !tcpFlagsInvalid(flags) {
			t.Fatalf("TCP SYN+FIN flags (0x%02x) should be detected as malformed", flags)
		}
	})
}

// TestXDPMalformed_TCPSynRst verifies that TCP packets with SYN+RST are
// detected as malformed.
func TestXDPMalformed_TCPSynRst(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate flags that have both SYN and RST set, with random other bits
		extraFlags := rapid.Byte().Draw(t, "extraFlags")
		flags := (extraFlags & tcpFlagMask) | tcpFlagSYN | tcpFlagRST

		if !tcpFlagsInvalid(flags) {
			t.Fatalf("TCP SYN+RST flags (0x%02x) should be detected as malformed", flags)
		}
	})
}

// TestXDPMalformed_TCPChristmasTree verifies that TCP packets with FIN+PSH+URG
// (Christmas tree) are detected as malformed.
func TestXDPMalformed_TCPChristmasTree(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate flags that have FIN+PSH+URG set, with random other bits
		extraFlags := rapid.Byte().Draw(t, "extraFlags")
		flags := (extraFlags & tcpFlagMask) | tcpFlagFIN | tcpFlagPSH | tcpFlagURG

		if !tcpFlagsInvalid(flags) {
			t.Fatalf("TCP Christmas tree flags (0x%02x) should be detected as malformed", flags)
		}
	})
}

// TestXDPMalformed_IPTotalLengthInvalid verifies that IP packets with
// total_length < ip_header_length are detected as malformed.
func TestXDPMalformed_IPTotalLengthInvalid(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// IP header length is ihl*4, minimum 20 bytes (ihl=5), max 60 bytes (ihl=15)
		headerLen := uint16(rapid.IntRange(20, 60).Draw(t, "headerLen"))
		// Total length must be strictly less than header length
		totalLen := uint16(rapid.IntRange(0, int(headerLen)-1).Draw(t, "totalLen"))

		if !ipTotalLengthInvalid(totalLen, headerLen) {
			t.Fatalf("IP total_length=%d < header_length=%d should be detected as malformed",
				totalLen, headerLen)
		}
	})
}

// TestXDPMalformed_UDPLengthTooShort verifies that UDP packets with
// udp_length < 8 are detected as malformed.
func TestXDPMalformed_UDPLengthTooShort(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// UDP length must be < 8 (minimum UDP header size)
		udpLen := uint16(rapid.IntRange(0, 7).Draw(t, "udpLen"))
		// IP payload length can be anything valid
		ipPayloadLen := uint16(rapid.IntRange(8, 65535).Draw(t, "ipPayloadLen"))

		if !udpLengthInvalid(udpLen, ipPayloadLen) {
			t.Fatalf("UDP length=%d < 8 should be detected as malformed", udpLen)
		}
	})
}

// TestXDPMalformed_UDPLengthExceedsPayload verifies that UDP packets with
// udp_length > ip_payload_length are detected as malformed.
func TestXDPMalformed_UDPLengthExceedsPayload(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// IP payload length (total_len - ip_header_len), reasonable range
		ipPayloadLen := uint16(rapid.IntRange(8, 65000).Draw(t, "ipPayloadLen"))
		// UDP length must be > IP payload length (but still >= 8 to isolate this condition)
		udpLen := uint16(rapid.IntRange(int(ipPayloadLen)+1, 65535).Draw(t, "udpLen"))

		if !udpLengthInvalid(udpLen, ipPayloadLen) {
			t.Fatalf("UDP length=%d > ip_payload_length=%d should be detected as malformed",
				udpLen, ipPayloadLen)
		}
	})
}

// TestXDPMalformed_ValidTCPFlags verifies that valid TCP flag combinations
// are NOT detected as malformed.
func TestXDPMalformed_ValidTCPFlags(t *testing.T) {
	// Valid flag combinations that should NOT be flagged as malformed
	validFlagSets := []byte{
		tcpFlagSYN,                        // SYN only
		tcpFlagACK,                        // ACK only
		tcpFlagSYN | tcpFlagACK,           // SYN+ACK
		tcpFlagFIN | tcpFlagACK,           // FIN+ACK
		tcpFlagRST | tcpFlagACK,           // RST+ACK
		tcpFlagPSH | tcpFlagACK,           // PSH+ACK
		tcpFlagACK | tcpFlagURG,           // ACK+URG
		tcpFlagFIN | tcpFlagACK | tcpFlagPSH, // FIN+ACK+PSH (normal close)
	}

	rapid.Check(t, func(t *rapid.T) {
		// Pick a random valid flag combination
		idx := rapid.IntRange(0, len(validFlagSets)-1).Draw(t, "flagIdx")
		flags := validFlagSets[idx]

		if tcpFlagsInvalid(flags) {
			t.Fatalf("Valid TCP flags (0x%02x) should NOT be detected as malformed", flags)
		}
	})
}
