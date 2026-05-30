package cookie

import (
	"encoding/base64"
	"testing"
	"time"
)

func TestNewCookieManagerV2(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!!")
	ttl := 5 * time.Minute
	mgr := NewCookieManagerV2(secret, ttl)

	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
	if mgr.defaultTTL != ttl {
		t.Errorf("expected defaultTTL=%v, got %v", ttl, mgr.defaultTTL)
	}
}

func TestGenerateCookie_Basic(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!!")
	mgr := NewCookieManagerV2(secret, 5*time.Minute)

	cookie, err := mgr.GenerateCookie("192.168.1.1", "ja3-fingerprint-abc", 5*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cookie == "" {
		t.Fatal("expected non-empty cookie")
	}

	// Verify base64url decoding produces 57 bytes
	decoded, err := base64.RawURLEncoding.DecodeString(cookie)
	if err != nil {
		t.Fatalf("cookie is not valid base64url: %v", err)
	}
	if len(decoded) != cookiePayloadSize {
		t.Errorf("expected payload size %d, got %d", cookiePayloadSize, len(decoded))
	}

	// Verify version byte
	if decoded[0] != cookieVersion {
		t.Errorf("expected version 0x%02x, got 0x%02x", cookieVersion, decoded[0])
	}
}

func TestV2GenerateCookie_EmptyIP(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!!")
	mgr := NewCookieManagerV2(secret, 5*time.Minute)

	_, err := mgr.GenerateCookie("", "fingerprint", 5*time.Minute)
	if err != ErrV2EmptyIP {
		t.Errorf("expected ErrV2EmptyIP, got %v", err)
	}
}

func TestV2GenerateCookie_EmptySecret(t *testing.T) {
	mgr := NewCookieManagerV2(nil, 5*time.Minute)

	_, err := mgr.GenerateCookie("192.168.1.1", "fingerprint", 5*time.Minute)
	if err != ErrV2EmptySecret {
		t.Errorf("expected ErrV2EmptySecret, got %v", err)
	}
}

func TestValidateCookie_Valid(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!!")
	mgr := NewCookieManagerV2(secret, 5*time.Minute)

	ip := "10.0.0.1"
	tlsFP := "ja3-hash-xyz"
	ttl := 5 * time.Minute

	cookie, err := mgr.GenerateCookie(ip, tlsFP, ttl)
	if err != nil {
		t.Fatalf("generate error: %v", err)
	}

	valid, remaining, err := mgr.ValidateCookie(cookie, ip, tlsFP)
	if err != nil {
		t.Fatalf("validate error: %v", err)
	}
	if !valid {
		t.Fatal("expected cookie to be valid")
	}
	// Remaining TTL should be close to 5 minutes (within 2 seconds tolerance)
	if remaining < 4*time.Minute+58*time.Second || remaining > 5*time.Minute+1*time.Second {
		t.Errorf("unexpected remaining TTL: %v", remaining)
	}
}

func TestV2ValidateCookie_IPMismatch(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!!")
	mgr := NewCookieManagerV2(secret, 5*time.Minute)

	cookie, err := mgr.GenerateCookie("10.0.0.1", "fingerprint", 5*time.Minute)
	if err != nil {
		t.Fatalf("generate error: %v", err)
	}

	valid, _, err := mgr.ValidateCookie(cookie, "10.0.0.2", "fingerprint")
	if valid {
		t.Fatal("expected cookie to be invalid for different IP")
	}
	if err != ErrV2IPMismatch {
		t.Errorf("expected ErrV2IPMismatch, got %v", err)
	}
}

func TestValidateCookie_TLSFingerprintMismatch(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!!")
	mgr := NewCookieManagerV2(secret, 5*time.Minute)

	cookie, err := mgr.GenerateCookie("10.0.0.1", "fingerprint-A", 5*time.Minute)
	if err != nil {
		t.Fatalf("generate error: %v", err)
	}

	valid, _, err := mgr.ValidateCookie(cookie, "10.0.0.1", "fingerprint-B")
	if valid {
		t.Fatal("expected cookie to be invalid for different TLS fingerprint")
	}
	if err != ErrV2TLSFPMismatch {
		t.Errorf("expected ErrV2TLSFPMismatch, got %v", err)
	}
}

func TestValidateCookie_EmptyTLSFingerprint_Fallback(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!!")
	mgr := NewCookieManagerV2(secret, 5*time.Minute)

	// Generate cookie with empty TLS fingerprint (zero hash)
	cookie, err := mgr.GenerateCookie("10.0.0.1", "", 5*time.Minute)
	if err != nil {
		t.Fatalf("generate error: %v", err)
	}

	// Validate with empty TLS fingerprint — should pass (IP-only binding)
	valid, _, err := mgr.ValidateCookie(cookie, "10.0.0.1", "")
	if err != nil {
		t.Fatalf("validate error: %v", err)
	}
	if !valid {
		t.Fatal("expected cookie to be valid with empty TLS fingerprint")
	}
}

func TestValidateCookie_EmptyTLSFingerprint_SkipCheck(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!!")
	mgr := NewCookieManagerV2(secret, 5*time.Minute)

	// Generate cookie WITH a TLS fingerprint
	cookie, err := mgr.GenerateCookie("10.0.0.1", "fingerprint-A", 5*time.Minute)
	if err != nil {
		t.Fatalf("generate error: %v", err)
	}

	// Validate with empty TLS fingerprint — should pass (fallback to IP-only)
	valid, _, err := mgr.ValidateCookie(cookie, "10.0.0.1", "")
	if err != nil {
		t.Fatalf("validate error: %v", err)
	}
	if !valid {
		t.Fatal("expected cookie to be valid when TLS fingerprint is unavailable (fallback)")
	}
}

func TestV2ValidateCookie_Expired(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!!")
	mgr := NewCookieManagerV2(secret, 5*time.Minute)

	now := time.Now()
	mgr.SetNowFunc(func() time.Time { return now })

	cookie, err := mgr.GenerateCookie("10.0.0.1", "fp", 1*time.Second)
	if err != nil {
		t.Fatalf("generate error: %v", err)
	}

	// Advance time past expiry
	mgr.SetNowFunc(func() time.Time { return now.Add(2 * time.Second) })

	valid, _, err := mgr.ValidateCookie(cookie, "10.0.0.1", "fp")
	if valid {
		t.Fatal("expected cookie to be expired")
	}
	if err != ErrV2CookieExpired {
		t.Errorf("expected ErrV2CookieExpired, got %v", err)
	}
}

func TestValidateCookie_InvalidBase64(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!!")
	mgr := NewCookieManagerV2(secret, 5*time.Minute)

	valid, _, err := mgr.ValidateCookie("not-valid-base64!!!", "10.0.0.1", "fp")
	if valid {
		t.Fatal("expected invalid")
	}
	if err != ErrV2InvalidCookieFormat {
		t.Errorf("expected ErrV2InvalidCookieFormat, got %v", err)
	}
}

func TestValidateCookie_WrongSize(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!!")
	mgr := NewCookieManagerV2(secret, 5*time.Minute)

	// Encode a payload that's too short
	short := base64.RawURLEncoding.EncodeToString([]byte("tooshort"))
	valid, _, err := mgr.ValidateCookie(short, "10.0.0.1", "fp")
	if valid {
		t.Fatal("expected invalid")
	}
	if err != ErrV2InvalidCookieFormat {
		t.Errorf("expected ErrV2InvalidCookieFormat, got %v", err)
	}
}

func TestValidateCookie_WrongVersion(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!!")
	mgr := NewCookieManagerV2(secret, 5*time.Minute)

	cookie, err := mgr.GenerateCookie("10.0.0.1", "fp", 5*time.Minute)
	if err != nil {
		t.Fatalf("generate error: %v", err)
	}

	// Tamper with version byte
	decoded, _ := base64.RawURLEncoding.DecodeString(cookie)
	decoded[0] = 0xFF
	tampered := base64.RawURLEncoding.EncodeToString(decoded)

	valid, _, err := mgr.ValidateCookie(tampered, "10.0.0.1", "fp")
	if valid {
		t.Fatal("expected invalid")
	}
	if err != ErrV2InvalidVersion {
		t.Errorf("expected ErrV2InvalidVersion, got %v", err)
	}
}

func TestValidateCookie_TamperedPayload(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!!")
	mgr := NewCookieManagerV2(secret, 5*time.Minute)

	cookie, err := mgr.GenerateCookie("10.0.0.1", "fp", 5*time.Minute)
	if err != nil {
		t.Fatalf("generate error: %v", err)
	}

	// Tamper with IP hash (byte 2)
	decoded, _ := base64.RawURLEncoding.DecodeString(cookie)
	decoded[2] ^= 0xFF
	tampered := base64.RawURLEncoding.EncodeToString(decoded)

	valid, _, err := mgr.ValidateCookie(tampered, "10.0.0.1", "fp")
	if valid {
		t.Fatal("expected invalid for tampered payload")
	}
	if err != ErrV2HMACMismatch {
		t.Errorf("expected ErrV2HMACMismatch, got %v", err)
	}
}

func TestShouldRefresh(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!!")
	mgr := NewCookieManagerV2(secret, 5*time.Minute)

	tests := []struct {
		name        string
		remaining   time.Duration
		total       time.Duration
		shouldRefr  bool
	}{
		{"90% remaining", 4*time.Minute + 30*time.Second, 5 * time.Minute, false},
		{"60% remaining", 3 * time.Minute, 5 * time.Minute, false},
		{"50% remaining", 2*time.Minute + 30*time.Second, 5 * time.Minute, false},
		{"49% remaining", 2*time.Minute + 27*time.Second, 5 * time.Minute, true},
		{"25% remaining", 1*time.Minute + 15*time.Second, 5 * time.Minute, true},
		{"0% remaining", 0, 5 * time.Minute, true},
		{"zero total TTL", 1 * time.Minute, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mgr.ShouldRefresh(tt.remaining, tt.total)
			if result != tt.shouldRefr {
				t.Errorf("ShouldRefresh(%v, %v) = %v, want %v", tt.remaining, tt.total, result, tt.shouldRefr)
			}
		})
	}
}

func TestGenerateCookie_UniqueNonce(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!!")
	mgr := NewCookieManagerV2(secret, 5*time.Minute)

	// Generate two cookies with same parameters — they should differ due to nonce
	cookie1, err := mgr.GenerateCookie("10.0.0.1", "fp", 5*time.Minute)
	if err != nil {
		t.Fatalf("generate error: %v", err)
	}
	cookie2, err := mgr.GenerateCookie("10.0.0.1", "fp", 5*time.Minute)
	if err != nil {
		t.Fatalf("generate error: %v", err)
	}

	if cookie1 == cookie2 {
		t.Error("expected different cookies due to unique nonce")
	}
}

func TestFnv1aHash32(t *testing.T) {
	// Verify deterministic hashing
	h1 := fnv1aHash32("test-string")
	h2 := fnv1aHash32("test-string")
	if h1 != h2 {
		t.Error("expected same hash for same input")
	}

	// Different inputs should produce different hashes
	h3 := fnv1aHash32("different-string")
	if h1 == h3 {
		t.Error("expected different hash for different input")
	}

	// Empty string should produce a valid hash (FNV offset basis)
	h4 := fnv1aHash32("")
	if h4 == 0 {
		t.Error("expected non-zero hash for empty string (FNV offset basis)")
	}
}
