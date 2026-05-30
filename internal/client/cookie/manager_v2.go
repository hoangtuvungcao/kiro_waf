// Package cookie triển khai HMAC-SHA256 access cookie cho Client_WAF.
// CookieManagerV2 adds TLS fingerprint binding and short-lived rotation.
package cookie

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/fnv"
	"time"
)

const (
	// cookieVersion is the current cookie format version.
	cookieVersion byte = 0x02

	// cookiePayloadSize is the total binary size before base64 encoding.
	// Version(1) + IP_Hash(4) + TLS_FP_Hash(4) + Expiry(8) + Nonce(8) + HMAC(32) = 57
	cookiePayloadSize = 57

	// hmacOffset is where the HMAC starts in the payload.
	hmacOffset = 1 + 4 + 4 + 8 + 8 // 25

	// hmacSize is the size of the HMAC-SHA256 output.
	hmacSize = 32
)

// Errors returned by CookieManagerV2.
var (
	ErrV2InvalidCookieFormat = errors.New("cookie_v2: invalid format")
	ErrV2InvalidVersion      = errors.New("cookie_v2: unsupported version")
	ErrV2HMACMismatch        = errors.New("cookie_v2: HMAC signature mismatch")
	ErrV2IPMismatch          = errors.New("cookie_v2: IP hash mismatch")
	ErrV2TLSFPMismatch       = errors.New("cookie_v2: TLS fingerprint hash mismatch")
	ErrV2CookieExpired       = errors.New("cookie_v2: expired")
	ErrV2EmptyIP             = errors.New("cookie_v2: empty IP address")
	ErrV2EmptySecret         = errors.New("cookie_v2: empty secret")
)

// CookieManagerV2 implements enhanced cookie management with TLS fingerprint
// binding and short-lived rotation support.
//
// Cookie binary format (57 bytes total):
//
//	Version(1B) | IP_Hash(4B, FNV-1a) | TLS_FP_Hash(4B, FNV-1a) | Expiry(8B) | Nonce(8B) | HMAC(32B)
//
// The cookie is base64url-encoded for transport (76 characters).
type CookieManagerV2 struct {
	secret     []byte
	defaultTTL time.Duration

	// nowFunc allows overriding time for testing. Defaults to time.Now.
	nowFunc func() time.Time

	// randFunc allows overriding random nonce generation for testing.
	randFunc func([]byte) (int, error)
}

// NewCookieManagerV2 creates a new CookieManagerV2 with the given secret and default TTL.
func NewCookieManagerV2(secret []byte, defaultTTL time.Duration) *CookieManagerV2 {
	return &CookieManagerV2{
		secret:     secret,
		defaultTTL: defaultTTL,
		nowFunc:    time.Now,
		randFunc:   rand.Read,
	}
}

// SetNowFunc overrides the time function for testing.
func (m *CookieManagerV2) SetNowFunc(fn func() time.Time) {
	m.nowFunc = fn
}

// SetRandFunc overrides the random nonce generator for testing.
func (m *CookieManagerV2) SetRandFunc(fn func([]byte) (int, error)) {
	m.randFunc = fn
}

// GenerateCookie creates an access cookie bound to the given IP and TLS fingerprint.
// If tlsFingerprint is empty, a zero hash is used for the TLS field (IP-only binding).
// The cookie is returned as a base64url-encoded string.
func (m *CookieManagerV2) GenerateCookie(ip, tlsFingerprint string, ttl time.Duration) (string, error) {
	if ip == "" {
		return "", ErrV2EmptyIP
	}
	if len(m.secret) == 0 {
		return "", ErrV2EmptySecret
	}

	// Build binary payload
	payload := make([]byte, cookiePayloadSize)

	// Version (1 byte)
	payload[0] = cookieVersion

	// IP Hash (4 bytes, FNV-1a)
	ipHash := fnv1aHash32(ip)
	binary.BigEndian.PutUint32(payload[1:5], ipHash)

	// TLS Fingerprint Hash (4 bytes, FNV-1a) — zero if empty
	var tlsHash uint32
	if tlsFingerprint != "" {
		tlsHash = fnv1aHash32(tlsFingerprint)
	}
	binary.BigEndian.PutUint32(payload[5:9], tlsHash)

	// Expiry (8 bytes, Unix timestamp in seconds)
	expiresAt := m.now().Add(ttl).Unix()
	binary.BigEndian.PutUint64(payload[9:17], uint64(expiresAt))

	// Nonce (8 bytes, random)
	if _, err := m.randFunc(payload[17:25]); err != nil {
		return "", err
	}

	// HMAC-SHA256 over first 25 bytes (Version + IP_Hash + TLS_FP_Hash + Expiry + Nonce)
	mac := hmac.New(sha256.New, m.secret)
	mac.Write(payload[:hmacOffset])
	copy(payload[hmacOffset:], mac.Sum(nil))

	// Base64url encode
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	return encoded, nil
}

// ValidateCookie verifies the cookie HMAC, IP binding, TLS fingerprint binding, and expiry.
// Returns (valid, remainingTTL, error).
// If tlsFingerprint is empty, the TLS field check is skipped (fallback to IP-only).
func (m *CookieManagerV2) ValidateCookie(cookie, ip, tlsFingerprint string) (bool, time.Duration, error) {
	if ip == "" {
		return false, 0, ErrV2EmptyIP
	}
	if len(m.secret) == 0 {
		return false, 0, ErrV2EmptySecret
	}

	// Decode base64url
	payload, err := base64.RawURLEncoding.DecodeString(cookie)
	if err != nil {
		return false, 0, ErrV2InvalidCookieFormat
	}

	if len(payload) != cookiePayloadSize {
		return false, 0, ErrV2InvalidCookieFormat
	}

	// Check version
	if payload[0] != cookieVersion {
		return false, 0, ErrV2InvalidVersion
	}

	// Verify HMAC first (constant-time comparison)
	mac := hmac.New(sha256.New, m.secret)
	mac.Write(payload[:hmacOffset])
	expectedMAC := mac.Sum(nil)
	if !hmac.Equal(payload[hmacOffset:], expectedMAC) {
		return false, 0, ErrV2HMACMismatch
	}

	// Verify IP hash
	storedIPHash := binary.BigEndian.Uint32(payload[1:5])
	expectedIPHash := fnv1aHash32(ip)
	if storedIPHash != expectedIPHash {
		return false, 0, ErrV2IPMismatch
	}

	// Verify TLS fingerprint hash
	storedTLSHash := binary.BigEndian.Uint32(payload[5:9])
	if tlsFingerprint != "" {
		// When TLS fingerprint is provided, it must match
		expectedTLSHash := fnv1aHash32(tlsFingerprint)
		if storedTLSHash != expectedTLSHash {
			return false, 0, ErrV2TLSFPMismatch
		}
	}
	// When tlsFingerprint is empty, skip TLS check (fallback to IP-only binding)

	// Check expiry
	expiryUnix := int64(binary.BigEndian.Uint64(payload[9:17]))
	now := m.now().Unix()
	if now > expiryUnix {
		return false, 0, ErrV2CookieExpired
	}

	remainingTTL := time.Duration(expiryUnix-now) * time.Second
	return true, remainingTTL, nil
}

// ShouldRefresh returns true when the remaining TTL is less than 50% of the total TTL,
// indicating the cookie should be refreshed.
func (m *CookieManagerV2) ShouldRefresh(remainingTTL, totalTTL time.Duration) bool {
	if totalTTL <= 0 {
		return false
	}
	return remainingTTL < totalTTL/2
}

// now returns the current time, using nowFunc if set.
func (m *CookieManagerV2) now() time.Time {
	if m.nowFunc != nil {
		return m.nowFunc()
	}
	return time.Now()
}

// fnv1aHash32 computes the FNV-1a 32-bit hash of a string.
func fnv1aHash32(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
