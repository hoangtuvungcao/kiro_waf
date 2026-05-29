// Feature: waf-system-overhaul, Property 10: XDP Per-IP Rate Limiting
// **Validates: Requirements 6.2**
//
// For any IPv4 address và cấu hình `per_ip_pps` threshold, sau khi gửi đúng
// `per_ip_pps` packets trong một cửa sổ thời gian, gói tin thứ `per_ip_pps + 1`
// SHALL bị drop. Các gói tin từ IP khác SHALL không bị ảnh hưởng.
package property

import (
	"testing"

	"pgregory.net/rapid"
)

// --- Go simulation of XDP per-IP rate limiting logic ---

// xdpRateAction represents the XDP rate limiting decision.
type xdpRateAction int

const (
	xdpRatePass     xdpRateAction = 0
	xdpRateDropIP   xdpRateAction = 1
)

// xdpIPRateState holds per-IP rate counter state within a time window.
type xdpIPRateState struct {
	windowStartNS uint64
	totalCount    uint32
}

// xdpIPRateLimiter simulates the XDP per-IP rate limiting logic.
// Each IP has an independent counter. When the counter exceeds the threshold
// within the configured window, subsequent packets are dropped.
type xdpIPRateLimiter struct {
	windowNS  uint64 // window duration in nanoseconds
	perIPPPS  uint32 // per-IP packets-per-second threshold
	state     map[uint32]*xdpIPRateState // keyed by IP address
	currentNS uint64 // simulated current time in nanoseconds
}

// newXDPIPRateLimiter creates a new rate limiter with the given configuration.
func newXDPIPRateLimiter(windowNS uint64, perIPPPS uint32) *xdpIPRateLimiter {
	return &xdpIPRateLimiter{
		windowNS:  windowNS,
		perIPPPS:  perIPPPS,
		state:     make(map[uint32]*xdpIPRateState),
		currentNS: 1000000000, // start at 1 second
	}
}

// checkPacket simulates the XDP rate_state_check logic for a single IP.
// Returns xdpRatePass if under threshold, xdpRateDropIP if exceeded.
// This mirrors the C code's rate_state_check function behavior:
// - First packet initializes state with count=1
// - Window reset when elapsed time >= windowNS
// - Drop when total_count > threshold (using threshold_exceeded: threshold > 0 && value > threshold)
func (r *xdpIPRateLimiter) checkPacket(srcIP uint32) xdpRateAction {
	// If threshold is 0, rate limiting is disabled
	if r.perIPPPS == 0 {
		return xdpRatePass
	}

	now := r.currentNS

	state, exists := r.state[srcIP]
	if !exists {
		// First packet from this IP — initialize state
		r.state[srcIP] = &xdpIPRateState{
			windowStartNS: now,
			totalCount:    1,
		}
		return xdpRatePass
	}

	// Reset counters if the current window has elapsed
	if r.windowNS == 0 || now-state.windowStartNS >= r.windowNS {
		state.windowStartNS = now
		state.totalCount = 0
	}

	// Increment counter
	state.totalCount++

	// Check threshold: drop when value > threshold
	if r.perIPPPS > 0 && state.totalCount > r.perIPPPS {
		return xdpRateDropIP
	}

	return xdpRatePass
}

// --- Property Tests ---

// TestXDPRateIP_ThresholdDrop verifies that after exactly `threshold` packets
// from an IP within a window, the next packet (threshold+1) is dropped.
func TestXDPRateIP_ThresholdDrop(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random threshold between 5 and 100
		threshold := rapid.Uint32Range(5, 100).Draw(t, "threshold")
		// Generate random IP address (any valid uint32)
		srcIP := rapid.Uint32().Draw(t, "srcIP")
		// Window of 1 second (1_000_000_000 ns)
		windowNS := uint64(1_000_000_000)

		limiter := newXDPIPRateLimiter(windowNS, threshold)

		// Send exactly `threshold` packets — all should pass
		for i := uint32(0); i < threshold; i++ {
			action := limiter.checkPacket(srcIP)
			if action != xdpRatePass {
				t.Fatalf("packet %d of %d was dropped (should pass): srcIP=0x%08x, threshold=%d",
					i+1, threshold, srcIP, threshold)
			}
		}

		// The (threshold+1)th packet SHALL be dropped
		action := limiter.checkPacket(srcIP)
		if action != xdpRateDropIP {
			t.Fatalf("packet %d (threshold+1) was NOT dropped: srcIP=0x%08x, threshold=%d",
				threshold+1, srcIP, threshold)
		}
	})
}

// TestXDPRateIP_IndependentCounters verifies that packets from a different IP
// are not affected by another IP's rate limit being exceeded.
func TestXDPRateIP_IndependentCounters(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random threshold between 5 and 100
		threshold := rapid.Uint32Range(5, 100).Draw(t, "threshold")
		// Generate two distinct IP addresses
		ipA := rapid.Uint32().Draw(t, "ipA")
		ipB := rapid.Uint32().Filter(func(v uint32) bool {
			return v != ipA
		}).Draw(t, "ipB")
		// Window of 1 second
		windowNS := uint64(1_000_000_000)

		limiter := newXDPIPRateLimiter(windowNS, threshold)

		// Exhaust IP_A's rate limit: send threshold+1 packets
		for i := uint32(0); i < threshold; i++ {
			limiter.checkPacket(ipA)
		}
		// Confirm IP_A is now rate-limited
		actionA := limiter.checkPacket(ipA)
		if actionA != xdpRateDropIP {
			t.Fatalf("IP_A (0x%08x) should be rate-limited after %d+1 packets, threshold=%d",
				ipA, threshold, threshold)
		}

		// IP_B should still be able to send packets freely
		for i := uint32(0); i < threshold; i++ {
			action := limiter.checkPacket(ipB)
			if action != xdpRatePass {
				t.Fatalf("IP_B (0x%08x) packet %d was dropped, but IP_A's rate limit should not affect IP_B. threshold=%d",
					ipB, i+1, threshold)
			}
		}
	})
}
