package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"kiro_waf/internal/master/plan"
)

// registerRequest represents the incoming registration payload.
type registerRequest struct {
	Hostname    string `json:"hostname"`
	Fingerprint string `json:"fingerprint"`
}

// registerResponse represents the response to a successful registration.
type registerResponse struct {
	LicenseKey string `json:"license_key"`
	LicenseID  string `json:"license_id"`
	Plan       string `json:"plan"`
	Status     string `json:"status"`
}

// rateLimitEntry tracks registration attempts per IP.
type rateLimitEntry struct {
	count     int
	resetTime time.Time
}

// registrationRateLimiter provides simple in-memory rate limiting per IP.
type registrationRateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rateLimitEntry
	limit   int
	window  time.Duration
}

// newRegistrationRateLimiter creates a rate limiter with the given limit per window.
func newRegistrationRateLimiter(limit int, window time.Duration) *registrationRateLimiter {
	return &registrationRateLimiter{
		entries: make(map[string]*rateLimitEntry),
		limit:   limit,
		window:  window,
	}
}

// allow checks if the IP is within the rate limit. Returns true if allowed.
func (rl *registrationRateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.entries[ip]

	if !exists || now.After(entry.resetTime) {
		// New window — reset counter.
		rl.entries[ip] = &rateLimitEntry{
			count:     1,
			resetTime: now.Add(rl.window),
		}
		return true
	}

	if entry.count >= rl.limit {
		return false
	}

	entry.count++
	return true
}

// HandleRegister returns an http.HandlerFunc for POST /api/v1/register.
// It creates a Community license automatically for new registrations.
// Rate-limited to 10 registrations per IP per hour.
func HandleRegister(pm plan.PlanManager) http.HandlerFunc {
	rl := newRegistrationRateLimiter(10, 1*time.Hour)

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Rate limit check.
		clientIP := extractClientIP(r)
		if !rl.allow(clientIP) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{
				"error": "rate limit exceeded, max 10 registrations per hour",
			})
			return
		}

		var req registerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
			return
		}

		// Validate required fields.
		if req.Hostname == "" || req.Fingerprint == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "hostname and fingerprint are required",
			})
			return
		}

		// Create Community license using PlanManager.
		// customerID = fingerprint (unique per machine), customerName = hostname.
		license, err := pm.CreateCommunityLicense(req.Fingerprint, req.Hostname, req.Fingerprint)
		if err != nil {
			log.Printf("register: failed to create community license: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to create license",
			})
			return
		}

		writeJSON(w, http.StatusOK, registerResponse{
			LicenseKey: license.LicenseKey,
			LicenseID:  license.LicenseID,
			Plan:       "community",
			Status:     "active",
		})
	}
}
