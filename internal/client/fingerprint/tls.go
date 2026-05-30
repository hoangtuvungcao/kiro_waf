package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
)

// TLSExtractor extracts TLS fingerprint information from HTTP request headers.
// It checks trusted upstream headers (X-TLS-Fingerprint, CF-JA3) to obtain
// a fingerprint hash representing the client's TLS connection profile.
type TLSExtractor struct{}

// NewTLSExtractor creates a new TLSExtractor instance.
func NewTLSExtractor() *TLSExtractor {
	return &TLSExtractor{}
}

// ExtractFingerprint extracts the TLS fingerprint from the request headers.
// It checks headers in priority order:
//  1. X-TLS-Fingerprint — a pre-computed fingerprint from a trusted upstream (e.g., nginx)
//  2. CF-JA3 — Cloudflare's JA3 fingerprint header
//
// Returns the fingerprint string (hashed if raw), or empty string if unavailable.
func (e *TLSExtractor) ExtractFingerprint(r *http.Request) string {
	// Priority 1: X-TLS-Fingerprint header (pre-computed by upstream)
	if fp := r.Header.Get("X-TLS-Fingerprint"); fp != "" {
		return fp
	}

	// Priority 2: CF-JA3 header (Cloudflare JA3 fingerprint)
	if ja3 := r.Header.Get("CF-JA3"); ja3 != "" {
		return hashFingerprint(ja3)
	}

	// No TLS fingerprint data available
	return ""
}

// hashFingerprint produces a SHA-256 hash of the raw fingerprint string.
// This normalizes varying fingerprint formats into a consistent hash.
func hashFingerprint(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
