package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"kiro_waf/master-server/db"
	"kiro_waf/master-server/models"
)

// heartbeatRequest represents the incoming heartbeat payload from a client node.
type heartbeatRequest struct {
	LicenseKey      string         `json:"license_key"`
	NodeID          string         `json:"node_id"`
	FingerprintHash string         `json:"fingerprint_hash"`
	Stats           map[string]any `json:"stats"`
}

// heartbeatResponse represents the response to a heartbeat request.
type heartbeatResponse struct {
	Valid     bool   `json:"valid"`
	Lock      bool   `json:"lock"`
	Status    string `json:"status,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

// updateCheckRequest represents the incoming update check payload.
type updateCheckRequest struct {
	Component      string `json:"component"`
	Channel        string `json:"channel"`
	CurrentVersion string `json:"current_version"`
}

// updateCheckResponse represents the response to an update check request.
type updateCheckResponse struct {
	UpdateAvailable bool           `json:"update_available"`
	Release         *releaseInfo   `json:"release,omitempty"`
}

// releaseInfo contains metadata about an available release.
type releaseInfo struct {
	Version     string `json:"version"`
	ArtifactURL string `json:"artifact_url"`
	SHA256      string `json:"sha256"`
	Notes       string `json:"notes"`
}

// healthResponse represents the response to a health check request.
type healthResponse struct {
	Status string `json:"status"`
}

// HandleHeartbeat returns an http.HandlerFunc for POST /api/v1/heartbeat.
// It validates the license key, logs the heartbeat, and returns the license status.
func HandleHeartbeat(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req heartbeatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
			return
		}

		// Validate required fields.
		if req.LicenseKey == "" || req.NodeID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
			return
		}

		// Lookup license by key.
		license, err := database.GetLicenseByKey(req.LicenseKey)
		if err != nil {
			log.Printf("heartbeat: db error looking up license: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		// Invalid license key → respond with valid: false, lock: true.
		if license == nil {
			writeJSON(w, http.StatusOK, heartbeatResponse{
				Valid: false,
				Lock:  true,
			})
			return
		}

		// Check license status — only "active" licenses are valid.
		now := time.Now().UTC()
		isActive := license.Status == "active" && license.ExpiresAt.After(now)

		if !isActive {
			writeJSON(w, http.StatusOK, heartbeatResponse{
				Valid: false,
				Lock:  true,
			})
			return
		}

		// Log the heartbeat.
		hb := &models.Heartbeat{
			LicenseID:       license.LicenseID,
			NodeID:          req.NodeID,
			ClientIP:        extractClientIP(r),
			FingerprintHash: req.FingerprintHash,
			Stats:           req.Stats,
			CreatedAt:       now,
		}
		if err := database.LogHeartbeat(hb); err != nil {
			log.Printf("heartbeat: db error logging heartbeat: %v", err)
			// Non-fatal: still respond with license status.
		}

		// Update last heartbeat timestamp on the license.
		if err := database.UpdateLicenseHeartbeat(license.LicenseID, now); err != nil {
			log.Printf("heartbeat: db error updating license heartbeat: %v", err)
		}

		writeJSON(w, http.StatusOK, heartbeatResponse{
			Valid:     true,
			Lock:      false,
			Status:    license.Status,
			ExpiresAt: license.ExpiresAt.Format(time.RFC3339),
		})
	}
}

// HandleUpdateCheck returns an http.HandlerFunc for POST /api/v1/update/check.
// It looks up the latest release for the given component+channel and returns metadata.
func HandleUpdateCheck(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req updateCheckRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
			return
		}

		// Validate required fields.
		if req.Component == "" || req.Channel == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
			return
		}

		// Lookup latest release for the component+channel.
		release, err := database.GetLatestRelease(req.Component, req.Channel)
		if err != nil {
			log.Printf("update/check: db error: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		// No release found or current version is already the latest.
		if release == nil || release.Version == req.CurrentVersion {
			writeJSON(w, http.StatusOK, updateCheckResponse{
				UpdateAvailable: false,
			})
			return
		}

		writeJSON(w, http.StatusOK, updateCheckResponse{
			UpdateAvailable: true,
			Release: &releaseInfo{
				Version:     release.Version,
				ArtifactURL: release.ArtifactURL,
				SHA256:      release.SHA256,
				Notes:       release.Notes,
			},
		})
	}
}

// HandleHealthz returns an http.HandlerFunc for GET /healthz.
func HandleHealthz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
	}
}

// writeJSON encodes v as JSON and writes it to the response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON: encode error: %v", err)
	}
}

// extractClientIP extracts the client IP from the request,
// checking X-Forwarded-For and X-Real-IP headers first.
func extractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain.
		for i, ch := range xff {
			if ch == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr (strip port).
	addr := r.RemoteAddr
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}
