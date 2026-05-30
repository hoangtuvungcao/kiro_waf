package challenge

import (
	"errors"
	"sync"
	"time"
)

// Errors returned by AntiReplayValidator.ConsumeToken.
var (
	ErrTokenNotFound = errors.New("token not found")
	ErrTokenExpired  = errors.New("token expired")
	ErrIPMismatch    = errors.New("client IP mismatch")
)

// antiReplayEntry stores metadata for a single-use challenge token.
type antiReplayEntry struct {
	ClientIP  string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// AntiReplayValidator enforces single-use semantics for challenge tokens.
// Each token is deleted on the first ConsumeToken call regardless of whether
// validation succeeds or fails, preventing any form of replay.
type AntiReplayValidator struct {
	mu    sync.Mutex
	items map[string]antiReplayEntry
}

// NewAntiReplayValidator creates a new AntiReplayValidator.
func NewAntiReplayValidator() *AntiReplayValidator {
	return &AntiReplayValidator{
		items: make(map[string]antiReplayEntry),
	}
}

// IssueToken generates a cryptographically random token bound to the given
// client IP with the specified TTL. Returns the token string and the UTC
// issuance timestamp.
func (v *AntiReplayValidator) IssueToken(clientIP string, ttl time.Duration) (token string, issuedAt time.Time) {
	token = randomToken()
	issuedAt = time.Now().UTC()
	entry := antiReplayEntry{
		ClientIP:  clientIP,
		IssuedAt:  issuedAt,
		ExpiresAt: issuedAt.Add(ttl),
	}
	v.mu.Lock()
	v.items[token] = entry
	v.mu.Unlock()
	return token, issuedAt
}

// IssueTokenAt is like IssueToken but uses a caller-provided timestamp
// (useful for testing).
func (v *AntiReplayValidator) IssueTokenAt(clientIP string, ttl time.Duration, issuedAt time.Time) (token string) {
	token = randomToken()
	entry := antiReplayEntry{
		ClientIP:  clientIP,
		IssuedAt:  issuedAt,
		ExpiresAt: issuedAt.Add(ttl),
	}
	v.mu.Lock()
	v.items[token] = entry
	v.mu.Unlock()
	return token
}

// ConsumeToken attempts to consume a token. The token is deleted from the
// store immediately on the first call regardless of whether validation
// succeeds or fails (single-use semantics per Requirement 8.1).
//
// Validation checks (in order):
//  1. Token exists → ErrTokenNotFound
//  2. Client IP matches → ErrIPMismatch
//  3. Token not expired → ErrTokenExpired
//
// On success, returns the issuance timestamp so the caller can enforce
// minimum solve time (Requirement 8.5).
func (v *AntiReplayValidator) ConsumeToken(token string, clientIP string) (issuedAt time.Time, err error) {
	v.mu.Lock()
	entry, exists := v.items[token]
	// Always delete on first attempt — single-use regardless of outcome.
	delete(v.items, token)
	v.mu.Unlock()

	if !exists {
		return time.Time{}, ErrTokenNotFound
	}

	if entry.ClientIP != clientIP {
		return entry.IssuedAt, ErrIPMismatch
	}

	if time.Now().UTC().After(entry.ExpiresAt) {
		return entry.IssuedAt, ErrTokenExpired
	}

	return entry.IssuedAt, nil
}

// ConsumeTokenAt is like ConsumeToken but uses a caller-provided "now"
// timestamp for expiry checking (useful for testing).
func (v *AntiReplayValidator) ConsumeTokenAt(token string, clientIP string, now time.Time) (issuedAt time.Time, err error) {
	v.mu.Lock()
	entry, exists := v.items[token]
	delete(v.items, token)
	v.mu.Unlock()

	if !exists {
		return time.Time{}, ErrTokenNotFound
	}

	if entry.ClientIP != clientIP {
		return entry.IssuedAt, ErrIPMismatch
	}

	if now.After(entry.ExpiresAt) {
		return entry.IssuedAt, ErrTokenExpired
	}

	return entry.IssuedAt, nil
}

// Has returns true if the token currently exists in the store (for testing).
func (v *AntiReplayValidator) Has(token string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	_, exists := v.items[token]
	return exists
}

// Cleanup removes expired entries to prevent unbounded memory growth.
func (v *AntiReplayValidator) Cleanup() {
	now := time.Now().UTC()
	v.mu.Lock()
	defer v.mu.Unlock()
	for token, entry := range v.items {
		if now.After(entry.ExpiresAt) {
			delete(v.items, token)
		}
	}
}

// Len returns the number of tokens currently in the store (for testing).
func (v *AntiReplayValidator) Len() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return len(v.items)
}
