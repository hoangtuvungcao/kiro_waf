// Feature: waf-system-overhaul, Property 3: Cookie HMAC Round-Trip và IP Binding
// **Validates: Requirements 1.3, 7.1, 7.5**
//
// For any IP address và cookie secret, việc tạo access cookie rồi xác thực với cùng IP
// SHALL thành công. For any IP_A ≠ IP_B, cookie được tạo cho IP_A SHALL bị từ chối khi
// xác thực với IP_B. For any cookie có expiration timestamp đã qua, xác thực SHALL thất bại
// bất kể IP có khớp hay không.
package property

import (
	"fmt"
	"testing"
	"time"

	"kiro_waf/internal/client/cookie"

	"pgregory.net/rapid"
)

// genIPv4 generates a random valid IPv4 address string.
func genIPv4(t *rapid.T, label string) string {
	a := rapid.IntRange(1, 254).Draw(t, label+"_octet1")
	b := rapid.IntRange(0, 255).Draw(t, label+"_octet2")
	c := rapid.IntRange(0, 255).Draw(t, label+"_octet3")
	d := rapid.IntRange(1, 254).Draw(t, label+"_octet4")
	return fmt.Sprintf("%d.%d.%d.%d", a, b, c, d)
}

// genSecret generates a random secret byte slice of 8-64 bytes.
func genSecret(t *rapid.T, label string) []byte {
	length := rapid.IntRange(8, 64).Draw(t, label+"_len")
	b := make([]byte, length)
	for i := range b {
		b[i] = byte(rapid.IntRange(0, 255).Draw(t, fmt.Sprintf("%s_byte%d", label, i)))
	}
	return b
}

// TestCookieHMAC_RoundTrip verifies that generating a cookie for an IP and validating
// with the same IP always succeeds.
func TestCookieHMAC_RoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		ip := genIPv4(t, "ip")
		secret := genSecret(t, "secret")
		// TTL between 1 minute and 24 hours
		ttlMinutes := rapid.IntRange(1, 1440).Draw(t, "ttl_minutes")
		ttl := time.Duration(ttlMinutes) * time.Minute

		mgr := cookie.NewHMACCookieManager()

		// Generate cookie
		cookieVal, err := mgr.GenerateCookie(ip, secret, ttl)
		if err != nil {
			t.Fatalf("GenerateCookie failed: %v", err)
		}
		if cookieVal == "" {
			t.Fatal("GenerateCookie returned empty cookie")
		}

		// Validate with same IP — must succeed
		valid, err := mgr.ValidateCookie(cookieVal, ip, secret)
		if err != nil {
			t.Fatalf("ValidateCookie failed for same IP: %v", err)
		}
		if !valid {
			t.Fatalf("ValidateCookie returned false for same IP: ip=%s", ip)
		}
	})
}

// TestCookieHMAC_IPBinding verifies that a cookie generated for IP_A is rejected
// when validated with a different IP_B.
func TestCookieHMAC_IPBinding(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		ipA := genIPv4(t, "ipA")
		ipB := genIPv4(t, "ipB")

		// Ensure IP_A != IP_B
		if ipA == ipB {
			t.Skip("generated same IP for A and B, skipping")
		}

		secret := genSecret(t, "secret")
		ttlMinutes := rapid.IntRange(1, 1440).Draw(t, "ttl_minutes")
		ttl := time.Duration(ttlMinutes) * time.Minute

		mgr := cookie.NewHMACCookieManager()

		// Generate cookie for IP_A
		cookieVal, err := mgr.GenerateCookie(ipA, secret, ttl)
		if err != nil {
			t.Fatalf("GenerateCookie failed: %v", err)
		}

		// Validate with IP_B — must fail with ErrIPMismatch
		valid, err := mgr.ValidateCookie(cookieVal, ipB, secret)
		if valid {
			t.Fatalf("ValidateCookie should reject cookie for different IP: ipA=%s, ipB=%s", ipA, ipB)
		}
		if err != cookie.ErrIPMismatch {
			t.Fatalf("expected ErrIPMismatch, got: %v (ipA=%s, ipB=%s)", err, ipA, ipB)
		}
	})
}

// TestCookieHMAC_Expiration verifies that a cookie with a past expiration timestamp
// is always rejected regardless of IP match.
func TestCookieHMAC_Expiration(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		ip := genIPv4(t, "ip")
		secret := genSecret(t, "secret")

		// Generate cookie with a time in the past
		pastOffsetSeconds := rapid.IntRange(60, 86400).Draw(t, "past_offset_seconds")
		pastTime := time.Now().Add(-time.Duration(pastOffsetSeconds) * time.Second)

		// Create a manager with a fixed past time for generation
		genMgr := cookie.NewHMACCookieManager()
		genMgr.SetNowFunc(func() time.Time { return pastTime })

		// Use a very short TTL so the cookie is already expired by now
		// TTL of 1 second means cookie expired at pastTime + 1s, which is still in the past
		ttl := time.Duration(rapid.IntRange(1, 30).Draw(t, "ttl_seconds")) * time.Second

		cookieVal, err := genMgr.GenerateCookie(ip, secret, ttl)
		if err != nil {
			t.Fatalf("GenerateCookie failed: %v", err)
		}

		// Validate with current time — must fail with ErrCookieExpired
		nowMgr := cookie.NewHMACCookieManager()
		valid, err := nowMgr.ValidateCookie(cookieVal, ip, secret)
		if valid {
			t.Fatalf("ValidateCookie should reject expired cookie: ip=%s, pastTime=%v, ttl=%v",
				ip, pastTime, ttl)
		}
		if err != cookie.ErrCookieExpired {
			t.Fatalf("expected ErrCookieExpired, got: %v", err)
		}
	})
}
