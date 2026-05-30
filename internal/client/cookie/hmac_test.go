package cookie

import (
	"testing"
	"time"
)

func TestNewHMACCookieManager(t *testing.T) {
	mgr := NewHMACCookieManager()
	if mgr == nil {
		t.Fatal("NewHMACCookieManager returned nil")
	}
}

func TestGenerateCookie_Success(t *testing.T) {
	mgr := NewHMACCookieManager()
	secret := []byte("test-secret-key-32bytes-long!!")
	ip := "192.168.1.100"
	ttl := 1 * time.Hour

	cookie, err := mgr.GenerateCookie(ip, secret, ttl)
	if err != nil {
		t.Fatalf("GenerateCookie failed: %v", err)
	}
	if cookie == "" {
		t.Fatal("GenerateCookie returned empty cookie")
	}
}

func TestGenerateCookie_EmptyIP(t *testing.T) {
	mgr := NewHMACCookieManager()
	secret := []byte("test-secret")

	_, err := mgr.GenerateCookie("", secret, time.Hour)
	if err != ErrEmptyIP {
		t.Fatalf("expected ErrEmptyIP, got: %v", err)
	}
}

func TestGenerateCookie_EmptySecret(t *testing.T) {
	mgr := NewHMACCookieManager()

	_, err := mgr.GenerateCookie("1.2.3.4", nil, time.Hour)
	if err != ErrEmptySecret {
		t.Fatalf("expected ErrEmptySecret, got: %v", err)
	}

	_, err = mgr.GenerateCookie("1.2.3.4", []byte{}, time.Hour)
	if err != ErrEmptySecret {
		t.Fatalf("expected ErrEmptySecret for empty slice, got: %v", err)
	}
}

func TestValidateCookie_RoundTrip(t *testing.T) {
	mgr := NewHMACCookieManager()
	secret := []byte("my-secret-key")
	ip := "10.0.0.1"
	ttl := 30 * time.Minute

	cookie, err := mgr.GenerateCookie(ip, secret, ttl)
	if err != nil {
		t.Fatalf("GenerateCookie failed: %v", err)
	}

	valid, err := mgr.ValidateCookie(cookie, ip, secret)
	if err != nil {
		t.Fatalf("ValidateCookie failed: %v", err)
	}
	if !valid {
		t.Fatal("ValidateCookie returned false for valid cookie")
	}
}

func TestValidateCookie_IPMismatch(t *testing.T) {
	mgr := NewHMACCookieManager()
	secret := []byte("my-secret-key")
	ipA := "10.0.0.1"
	ipB := "10.0.0.2"
	ttl := 30 * time.Minute

	cookie, err := mgr.GenerateCookie(ipA, secret, ttl)
	if err != nil {
		t.Fatalf("GenerateCookie failed: %v", err)
	}

	valid, err := mgr.ValidateCookie(cookie, ipB, secret)
	if valid {
		t.Fatal("ValidateCookie should reject cookie with different IP")
	}
	if err != ErrIPMismatch {
		t.Fatalf("expected ErrIPMismatch, got: %v", err)
	}
}

func TestValidateCookie_Expired(t *testing.T) {
	// Use a fixed time that's in the past
	fixedTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	mgr := &HMACCookieManager{
		nowFunc: func() time.Time { return fixedTime },
	}
	secret := []byte("my-secret-key")
	ip := "10.0.0.1"
	ttl := 1 * time.Hour

	// Generate cookie with past time (expires at 2020-01-01 01:00:00)
	cookie, err := mgr.GenerateCookie(ip, secret, ttl)
	if err != nil {
		t.Fatalf("GenerateCookie failed: %v", err)
	}

	// Now validate with current time (well past expiry)
	mgrNow := NewHMACCookieManager()
	valid, err := mgrNow.ValidateCookie(cookie, ip, secret)
	if valid {
		t.Fatal("ValidateCookie should reject expired cookie")
	}
	if err != ErrCookieExpired {
		t.Fatalf("expected ErrCookieExpired, got: %v", err)
	}
}

func TestValidateCookie_InvalidFormat(t *testing.T) {
	mgr := NewHMACCookieManager()
	secret := []byte("my-secret-key")
	ip := "10.0.0.1"

	tests := []struct {
		name   string
		cookie string
	}{
		{"empty string", ""},
		{"not base64", "!!!invalid-base64!!!"},
		{"no colons", "aGVsbG8="}, // base64("hello")
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			valid, err := mgr.ValidateCookie(tc.cookie, ip, secret)
			if valid {
				t.Fatal("ValidateCookie should reject invalid format")
			}
			if err == nil {
				t.Fatal("expected error for invalid format")
			}
		})
	}
}

func TestValidateCookie_WrongSecret(t *testing.T) {
	mgr := NewHMACCookieManager()
	secretA := []byte("secret-A")
	secretB := []byte("secret-B")
	ip := "10.0.0.1"
	ttl := 1 * time.Hour

	cookie, err := mgr.GenerateCookie(ip, secretA, ttl)
	if err != nil {
		t.Fatalf("GenerateCookie failed: %v", err)
	}

	valid, err := mgr.ValidateCookie(cookie, ip, secretB)
	if valid {
		t.Fatal("ValidateCookie should reject cookie with wrong secret")
	}
	if err != ErrHMACMismatch {
		t.Fatalf("expected ErrHMACMismatch, got: %v", err)
	}
}

func TestValidateCookie_EmptyIP(t *testing.T) {
	mgr := NewHMACCookieManager()
	secret := []byte("my-secret-key")

	_, err := mgr.ValidateCookie("some-cookie", "", secret)
	if err != ErrEmptyIP {
		t.Fatalf("expected ErrEmptyIP, got: %v", err)
	}
}

func TestValidateCookie_EmptySecret(t *testing.T) {
	mgr := NewHMACCookieManager()

	_, err := mgr.ValidateCookie("some-cookie", "1.2.3.4", nil)
	if err != ErrEmptySecret {
		t.Fatalf("expected ErrEmptySecret, got: %v", err)
	}
}

func TestValidateCookie_IPv6RoundTrip(t *testing.T) {
	mgr := NewHMACCookieManager()
	secret := []byte("ipv6-secret-key")
	ip := "2001:db8::1"
	ttl := 1 * time.Hour

	cookie, err := mgr.GenerateCookie(ip, secret, ttl)
	if err != nil {
		t.Fatalf("GenerateCookie failed for IPv6: %v", err)
	}

	valid, err := mgr.ValidateCookie(cookie, ip, secret)
	if err != nil {
		t.Fatalf("ValidateCookie failed for IPv6: %v", err)
	}
	if !valid {
		t.Fatal("ValidateCookie returned false for valid IPv6 cookie")
	}
}

func TestValidateCookie_IPv6IPMismatch(t *testing.T) {
	mgr := NewHMACCookieManager()
	secret := []byte("ipv6-secret-key")
	ipA := "2001:db8::1"
	ipB := "2001:db8::2"
	ttl := 1 * time.Hour

	cookie, err := mgr.GenerateCookie(ipA, secret, ttl)
	if err != nil {
		t.Fatalf("GenerateCookie failed: %v", err)
	}

	valid, err := mgr.ValidateCookie(cookie, ipB, secret)
	if valid {
		t.Fatal("ValidateCookie should reject IPv6 cookie with different IP")
	}
	if err != ErrIPMismatch {
		t.Fatalf("expected ErrIPMismatch, got: %v", err)
	}
}

func TestCookieManager_ImplementsInterface(t *testing.T) {
	// Compile-time check that HMACCookieManager implements CookieManager
	var _ CookieManager = (*HMACCookieManager)(nil)
}

func TestValidateCookie_NotYetExpired(t *testing.T) {
	mgr := NewHMACCookieManager()
	secret := []byte("test-secret")
	ip := "1.2.3.4"
	ttl := 24 * time.Hour

	cookie, err := mgr.GenerateCookie(ip, secret, ttl)
	if err != nil {
		t.Fatalf("GenerateCookie failed: %v", err)
	}

	// Validate immediately - should be valid
	valid, err := mgr.ValidateCookie(cookie, ip, secret)
	if err != nil {
		t.Fatalf("ValidateCookie failed: %v", err)
	}
	if !valid {
		t.Fatal("cookie should be valid before expiry")
	}
}

func TestValidateCookie_ExactExpiry(t *testing.T) {
	// Cookie that expires exactly now should still be valid (now <= expiry)
	now := time.Now()
	mgr := &HMACCookieManager{
		nowFunc: func() time.Time { return now },
	}
	secret := []byte("test-secret")
	ip := "1.2.3.4"

	// Generate with 0 TTL - expires at exactly "now"
	cookie, err := mgr.GenerateCookie(ip, secret, 0)
	if err != nil {
		t.Fatalf("GenerateCookie failed: %v", err)
	}

	// Validate at the same time - now.Unix() == expiry, so not expired
	valid, err := mgr.ValidateCookie(cookie, ip, secret)
	if err != nil {
		t.Fatalf("ValidateCookie failed: %v", err)
	}
	if !valid {
		t.Fatal("cookie at exact expiry time should still be valid")
	}
}
