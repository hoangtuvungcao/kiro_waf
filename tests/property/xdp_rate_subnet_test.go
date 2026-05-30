// Feature: waf-system-overhaul, Property 11: XDP Per-Subnet Rate Limiting Independence
// **Validates: Requirements 6.3, 7.4**
//
// For any subnet /24 và cấu hình `per_subnet24_pps` threshold, khi tổng packets từ tất cả
// IP trong subnet vượt threshold, tất cả gói tin tiếp theo từ subnet đó SHALL bị drop —
// ngay cả khi mỗi IP riêng lẻ nằm dưới ngưỡng per-IP.
package property

import (
	"fmt"
	"testing"
	"time"

	"kiro_waf/internal/client/ratelimit"

	"pgregory.net/rapid"
)

// xdpSubnetRateLimiter simulates XDP per-subnet /24 rate limiting behavior.
// It tracks total packets per /24 subnet and drops all subsequent packets from
// that subnet once the threshold is exceeded, even if individual IPs are under
// the per-IP threshold.
type xdpSubnetRateLimiter struct {
	perIPPPS       uint32
	perSubnet24PPS uint32
	windowDuration time.Duration
	limiter        *ratelimit.SlidingWindowLimiter
}

// newXDPSubnetRateLimiter creates a new XDP subnet rate limiter simulation.
func newXDPSubnetRateLimiter(perIPPPS, perSubnet24PPS uint32, window time.Duration) *xdpSubnetRateLimiter {
	config := ratelimit.LimiterConfig{
		SoftThreshold:   int(perIPPPS),
		HardThreshold:   int(perIPPPS) * 2, // Hard threshold higher than soft
		SubnetThreshold: int(perSubnet24PPS),
		WindowDuration:  window,
	}
	return &xdpSubnetRateLimiter{
		perIPPPS:       perIPPPS,
		perSubnet24PPS: perSubnet24PPS,
		windowDuration: window,
		limiter:        ratelimit.NewSlidingWindowLimiter(config),
	}
}

// processPacket simulates XDP processing a packet from the given IP.
// Returns true if the packet is passed (XDP_PASS), false if dropped (XDP_DROP).
// Drop occurs if per-IP threshold is exceeded OR per-subnet threshold is exceeded.
func (x *xdpSubnetRateLimiter) processPacket(ip string) bool {
	subnet := x.limiter.GetSubnet24(ip)

	// Check subnet threshold first (aggregated across all IPs in /24)
	if !x.limiter.AllowSubnet(subnet) {
		return false // XDP_DROP - subnet rate exceeded
	}

	// Check per-IP threshold
	if !x.limiter.Allow(ip) {
		return false // XDP_DROP - per-IP rate exceeded
	}

	// Record the packet (updates both per-IP and per-subnet counters)
	x.limiter.RecordRequest(ip)
	return true // XDP_PASS
}

// TestXDPRateSubnet_DistributedTrafficExceedsSubnetThreshold verifies that when
// distributed traffic from multiple IPs in the same /24 subnet exceeds the
// per-subnet threshold, all subsequent packets from that subnet are dropped,
// even if each individual IP is under the per-IP threshold.
func TestXDPRateSubnet_DistributedTrafficExceedsSubnetThreshold(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random subnet (first 3 octets)
		octet1 := rapid.IntRange(1, 223).Draw(t, "octet1")
		octet2 := rapid.IntRange(0, 255).Draw(t, "octet2")
		octet3 := rapid.IntRange(0, 255).Draw(t, "octet3")

		// Generate per-subnet threshold (small enough to test quickly)
		perSubnet24PPS := uint32(rapid.IntRange(5, 50).Draw(t, "perSubnet24PPS"))

		// Per-IP threshold must be higher than per-subnet / numIPs to ensure
		// individual IPs stay under per-IP limit while subnet exceeds its limit
		numIPs := rapid.IntRange(3, 10).Draw(t, "numIPs")
		// Set per-IP threshold high enough that no single IP exceeds it
		perIPPPS := uint32(rapid.IntRange(int(perSubnet24PPS), int(perSubnet24PPS)*2).Draw(t, "perIPPPS"))

		window := 60 * time.Second
		limiter := newXDPSubnetRateLimiter(perIPPPS, perSubnet24PPS, window)

		// Generate IPs within the same /24 subnet
		ips := make([]string, numIPs)
		for i := 0; i < numIPs; i++ {
			lastOctet := i + 1 // Use sequential IPs: .1, .2, .3, ...
			if lastOctet > 254 {
				lastOctet = 254
			}
			ips[i] = fmt.Sprintf("%d.%d.%d.%d", octet1, octet2, octet3, lastOctet)
		}

		// Distribute packets evenly across IPs until subnet threshold is reached
		packetsPerIP := int(perSubnet24PPS) / numIPs
		if packetsPerIP < 1 {
			packetsPerIP = 1
		}

		totalSent := 0
		for _, ip := range ips {
			for j := 0; j < packetsPerIP; j++ {
				if totalSent >= int(perSubnet24PPS) {
					break
				}
				limiter.processPacket(ip)
				totalSent++
			}
			if totalSent >= int(perSubnet24PPS) {
				break
			}
		}

		// If we haven't reached the threshold yet, send remaining from first IPs
		for i := 0; totalSent < int(perSubnet24PPS); i++ {
			limiter.processPacket(ips[i%numIPs])
			totalSent++
		}

		// Now the subnet threshold is reached. Verify that ALL subsequent packets
		// from ANY IP in this subnet are dropped.
		for _, ip := range ips {
			result := limiter.processPacket(ip)
			if result {
				t.Fatalf("expected XDP_DROP for IP %s after subnet %d.%d.%d.0/24 exceeded threshold %d (total sent: %d), but got XDP_PASS",
					ip, octet1, octet2, octet3, perSubnet24PPS, totalSent)
			}
		}

		// Verify each individual IP sent fewer packets than per-IP threshold
		// (confirming the drop is due to subnet threshold, not per-IP threshold)
		maxPerIP := packetsPerIP + 1 // +1 for the verification packet
		if uint32(maxPerIP) >= perIPPPS {
			t.Skip("per-IP packets too close to per-IP threshold, skipping verification")
		}
	})
}

// TestXDPRateSubnet_DifferentSubnetNotAffected verifies that when one /24 subnet
// exceeds its rate limit, traffic from a different /24 subnet is NOT affected.
func TestXDPRateSubnet_DifferentSubnetNotAffected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate two different subnets
		octet1A := rapid.IntRange(1, 223).Draw(t, "octet1A")
		octet2A := rapid.IntRange(0, 255).Draw(t, "octet2A")
		octet3A := rapid.IntRange(0, 255).Draw(t, "octet3A")

		// Ensure subnet B is different from subnet A
		octet1B := rapid.IntRange(1, 223).Draw(t, "octet1B")
		octet2B := rapid.IntRange(0, 255).Draw(t, "octet2B")
		octet3B := rapid.IntRange(0, 255).Draw(t, "octet3B")

		// Skip if subnets happen to be the same
		if octet1A == octet1B && octet2A == octet2B && octet3A == octet3B {
			t.Skip("generated same subnet, skipping")
		}

		// Generate threshold
		perSubnet24PPS := uint32(rapid.IntRange(5, 50).Draw(t, "perSubnet24PPS"))
		perIPPPS := uint32(rapid.IntRange(int(perSubnet24PPS)*2, int(perSubnet24PPS)*4).Draw(t, "perIPPPS"))

		window := 60 * time.Second
		limiter := newXDPSubnetRateLimiter(perIPPPS, perSubnet24PPS, window)

		// Flood subnet A past its threshold using multiple IPs
		numIPsA := rapid.IntRange(2, 8).Draw(t, "numIPsA")
		totalSentA := 0
		for i := 0; totalSentA < int(perSubnet24PPS); i++ {
			lastOctet := (i % numIPsA) + 1
			ip := fmt.Sprintf("%d.%d.%d.%d", octet1A, octet2A, octet3A, lastOctet)
			limiter.processPacket(ip)
			totalSentA++
		}

		// Verify subnet A is now blocked
		ipA := fmt.Sprintf("%d.%d.%d.%d", octet1A, octet2A, octet3A, 200)
		resultA := limiter.processPacket(ipA)
		if resultA {
			t.Fatalf("expected subnet A (%d.%d.%d.0/24) to be blocked after %d packets, but got XDP_PASS",
				octet1A, octet2A, octet3A, totalSentA)
		}

		// Verify subnet B is NOT affected — packets should still pass
		ipB := fmt.Sprintf("%d.%d.%d.%d", octet1B, octet2B, octet3B, 1)
		resultB := limiter.processPacket(ipB)
		if !resultB {
			t.Fatalf("expected subnet B (%d.%d.%d.0/24) to still allow traffic after subnet A was blocked, but got XDP_DROP",
				octet1B, octet2B, octet3B)
		}

		// Send a few more packets from subnet B to confirm it's still working
		numExtraB := rapid.IntRange(1, 3).Draw(t, "numExtraB")
		for i := 0; i < numExtraB; i++ {
			lastOctet := i + 2
			ipExtra := fmt.Sprintf("%d.%d.%d.%d", octet1B, octet2B, octet3B, lastOctet)
			result := limiter.processPacket(ipExtra)
			if !result {
				t.Fatalf("expected subnet B IP %s to still pass, but got XDP_DROP (only %d packets sent to subnet B)",
					ipExtra, i+2)
			}
		}
	})
}
