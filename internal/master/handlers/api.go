package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"kiro_waf/internal/master/db"
	"kiro_waf/internal/master/models"
)

// heartbeatRequest represents the incoming heartbeat payload from a client node.
type heartbeatRequest struct {
	LicenseKey      string         `json:"license_key"`
	NodeID          string         `json:"node_id"`
	FingerprintHash string         `json:"fingerprint_hash"`
	Stats           map[string]any `json:"stats"`
}

// planConfigResponse is the plan configuration included in heartbeat responses.
type planConfigResponse struct {
	RPMPerIP   int  `json:"rpm_per_ip"`
	SubnetRPM  int  `json:"subnet_rpm"`
	MaxDomains int  `json:"max_domains"`
	XDPEnabled bool `json:"xdp_enabled"`
	OTAEnabled bool `json:"ota_enabled"`
}

// heartbeatResponse represents the response to a heartbeat request.
// Includes Package_Plan and license status for client-side enforcement.
type heartbeatResponse struct {
	Valid       bool                `json:"valid"`
	Lock        bool                `json:"lock"`
	Status      string              `json:"status,omitempty"`
	Plan        string              `json:"plan,omitempty"`
	ExpiresAt   string              `json:"expires_at,omitempty"`
	PlanConfig  *planConfigResponse `json:"plan_config,omitempty"`
	Reason      string              `json:"reason,omitempty"`
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

		ctx := r.Context()

		// Lookup license by key.
		license, err := database.GetLicenseByKeyCtx(ctx, req.LicenseKey)
		if err != nil {
			if IsContextTimeout(err) {
				WriteDBTimeoutResponse(w)
				return
			}
			log.Printf("heartbeat: db error looking up license: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		// Fallback: if not found by license_key, try by license_id.
		// This handles the case where admin panel shows license_id to users.
		if license == nil {
			license, _ = database.GetLicenseByLicenseID(req.LicenseKey)
		}

		// Invalid license key → respond with valid: false, lock: true.
		if license == nil {
			writeJSON(w, http.StatusOK, heartbeatResponse{
				Valid: false,
				Lock:  true,
			})
			return
		}

		// Check license status.
		now := time.Now().UTC()

		// Revoked license → hard lock (cannot operate at all).
		if license.Status == "revoked" {
			writeJSON(w, http.StatusOK, heartbeatResponse{
				Valid:  false,
				Lock:   true,
				Status: "revoked",
				Plan:   license.Plan,
				Reason: "license_revoked",
			})
			return
		}

		// Suspended license → signal suspension (client stops traffic, shows suspension page).
		// Per Requirement 14.7/14.10: suspended is the only state that prevents Client_Node from operating.
		if license.Status == "suspended" {
			writeJSON(w, http.StatusOK, heartbeatResponse{
				Valid:  false,
				Lock:   false,
				Status: "suspended",
				Plan:   license.Plan,
				Reason: "license_suspended",
			})
			return
		}

		// Check license expiry — auto-downgrade to community instead of locking.
		// Zero ExpiresAt means perpetual (Community plan) — never expires.
		expired := !license.ExpiresAt.IsZero() && license.ExpiresAt.Before(now)
		if expired && license.Status == "active" {
			// Mark license as expired in DB.
			license.Status = "expired"
			if err := database.UpdateLicenseStatusCtx(ctx, license.LicenseID, "expired"); err != nil {
				if !IsContextTimeout(err) {
					log.Printf("heartbeat: db error updating license status to expired: %v", err)
				}
			}
		}

		// Binary integrity check: if fingerprint_hash is set on the license,
		// verify it matches the client's reported hash.
		if license.FingerprintHash != "" && req.FingerprintHash != "" &&
			license.FingerprintHash != req.FingerprintHash {
			log.Printf("heartbeat: binary integrity mismatch for license=%s node=%s expected=%s got=%s",
				license.LicenseID, req.NodeID, license.FingerprintHash, req.FingerprintHash)
			writeJSON(w, http.StatusOK, heartbeatResponse{
				Valid:  false,
				Lock:   true,
				Reason: "binary_integrity_mismatch",
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
		if err := database.LogHeartbeatCtx(ctx, hb); err != nil {
			if IsContextTimeout(err) {
				WriteDBTimeoutResponse(w)
				return
			}
			log.Printf("heartbeat: db error logging heartbeat: %v", err)
		}

		// Update last heartbeat timestamp on the license.
		if err := database.UpdateLicenseHeartbeatCtx(ctx, license.LicenseID, now); err != nil {
			if IsContextTimeout(err) {
				WriteDBTimeoutResponse(w)
				return
			}
			log.Printf("heartbeat: db error updating license heartbeat: %v", err)
		}

		// Build plan config response.
		// If expired, downgrade to community plan config.
		planName := license.Plan
		if expired {
			planName = "community"
		}

		var pc *planConfigResponse
		if cfg, ok := models.PlanConfigs[planName]; ok {
			pc = &planConfigResponse{
				RPMPerIP:   cfg.RPMPerIP,
				SubnetRPM:  cfg.SubnetRPM,
				MaxDomains: cfg.MaxDomains,
				XDPEnabled: cfg.XDPEnabled,
				OTAEnabled: cfg.OTAEnabled,
			}
		}

		status := license.Status
		if expired {
			status = "expired"
		}

		writeJSON(w, http.StatusOK, heartbeatResponse{
			Valid:      !expired,
			Lock:       false,
			Status:     status,
			Plan:       planName,
			ExpiresAt:  license.ExpiresAt.Format(time.RFC3339),
			PlanConfig: pc,
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

		ctx := r.Context()

		// Lookup latest release for the component+channel.
		release, err := database.GetLatestReleaseCtx(ctx, req.Component, req.Channel)
		if err != nil {
			if IsContextTimeout(err) {
				WriteDBTimeoutResponse(w)
				return
			}
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
// checking CF-Connecting-IP, X-Forwarded-For and X-Real-IP headers first.
func extractClientIP(r *http.Request) string {
	// Cloudflare sends the real client IP in this header.
	if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" {
		return cfIP
	}
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
