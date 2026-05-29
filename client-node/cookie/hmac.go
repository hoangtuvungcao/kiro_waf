// Package cookie triển khai HMAC-SHA256 access cookie cho Client_WAF.
package cookie

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Errors returned by HMACCookieManager.
var (
	ErrInvalidCookieFormat = errors.New("cookie: invalid format")
	ErrHMACMismatch        = errors.New("cookie: HMAC signature mismatch")
	ErrIPMismatch          = errors.New("cookie: IP address mismatch")
	ErrCookieExpired       = errors.New("cookie: expired")
	ErrEmptyIP             = errors.New("cookie: empty IP address")
	ErrEmptySecret         = errors.New("cookie: empty secret")
)

// HMACCookieManager implements CookieManager using HMAC-SHA256.
// Cookie format: base64url(ip + ":" + expiry_unix + ":" + hmac_hex)
// HMAC is computed over: ip + ":" + expiry_unix
type HMACCookieManager struct {
	// nowFunc allows overriding time for testing. Defaults to time.Now.
	nowFunc func() time.Time
}

// NewHMACCookieManager creates a new HMACCookieManager instance.
func NewHMACCookieManager() *HMACCookieManager {
	return &HMACCookieManager{
		nowFunc: time.Now,
	}
}

// SetNowFunc overrides the time function used by the cookie manager.
// This is intended for testing purposes to simulate time-based scenarios.
func (m *HMACCookieManager) SetNowFunc(fn func() time.Time) {
	m.nowFunc = fn
}

// GenerateCookie creates an access cookie binding the given IP with an expiration.
// Cookie format: base64url(ip:expiry_unix:hmac_hex)
// HMAC-SHA256 is computed over "ip:expiry_unix" using the provided secret.
func (m *HMACCookieManager) GenerateCookie(ip string, secret []byte, ttl time.Duration) (string, error) {
	if ip == "" {
		return "", ErrEmptyIP
	}
	if len(secret) == 0 {
		return "", ErrEmptySecret
	}

	expiresAt := m.now().Add(ttl)
	expiryStr := strconv.FormatInt(expiresAt.Unix(), 10)

	// Compute HMAC-SHA256 over "ip:expiry_unix"
	payload := ip + ":" + expiryStr
	sig := computeHMAC([]byte(payload), secret)
	sigHex := hex.EncodeToString(sig)

	// Cookie value: base64url(ip:expiry_unix:hmac_hex)
	cookieRaw := fmt.Sprintf("%s:%s:%s", ip, expiryStr, sigHex)
	cookieEncoded := base64.URLEncoding.EncodeToString([]byte(cookieRaw))

	return cookieEncoded, nil
}

// ValidateCookie verifies the HMAC signature, checks IP match, and checks expiration.
// Returns true if the cookie is valid, false otherwise.
func (m *HMACCookieManager) ValidateCookie(cookie string, ip string, secret []byte) (bool, error) {
	if ip == "" {
		return false, ErrEmptyIP
	}
	if len(secret) == 0 {
		return false, ErrEmptySecret
	}

	// Decode base64url
	decoded, err := base64.URLEncoding.DecodeString(cookie)
	if err != nil {
		return false, ErrInvalidCookieFormat
	}

	// Parse "ip:expiry_unix:hmac_hex"
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) < 2 {
		return false, ErrInvalidCookieFormat
	}

	// The IP may contain colons (IPv6), so we need to find the last two colons
	// Format: ip:expiry_unix:hmac_hex
	// Strategy: find the hmac (last 64 chars hex = SHA256), then expiry before it
	raw := string(decoded)
	lastColon := strings.LastIndex(raw, ":")
	if lastColon < 0 {
		return false, ErrInvalidCookieFormat
	}
	sigHex := raw[lastColon+1:]

	remaining := raw[:lastColon]
	secondLastColon := strings.LastIndex(remaining, ":")
	if secondLastColon < 0 {
		return false, ErrInvalidCookieFormat
	}

	cookieIP := remaining[:secondLastColon]
	expiryStr := remaining[secondLastColon+1:]

	// Verify IP match
	if cookieIP != ip {
		return false, ErrIPMismatch
	}

	// Parse expiry timestamp
	expiryUnix, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil {
		return false, ErrInvalidCookieFormat
	}

	// Check expiration
	if m.now().Unix() > expiryUnix {
		return false, ErrCookieExpired
	}

	// Verify HMAC signature
	payload := cookieIP + ":" + expiryStr
	expectedSig := computeHMAC([]byte(payload), secret)
	expectedHex := hex.EncodeToString(expectedSig)

	if !hmac.Equal([]byte(sigHex), []byte(expectedHex)) {
		return false, ErrHMACMismatch
	}

	return true, nil
}

// now returns the current time, using nowFunc if set.
func (m *HMACCookieManager) now() time.Time {
	if m.nowFunc != nil {
		return m.nowFunc()
	}
	return time.Now()
}

// computeHMAC computes HMAC-SHA256 of data using the given key.
func computeHMAC(data, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}
