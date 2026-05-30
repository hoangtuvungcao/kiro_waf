package handlers

import (
	"log"
	"net/http"

	"kiro_waf/internal/master/db"
)

// dashboardChartData represents the JSON response for GET /api/v1/charts/dashboard.
type dashboardChartData struct {
	LicenseDistribution map[string]int `json:"license_distribution"`
	HeartbeatTimeline   []db.HourlyCount `json:"heartbeat_timeline"`
}

// releaseChartData represents the JSON response for GET /api/v1/charts/releases.
type releaseChartData struct {
	Releases []db.ReleasePoint `json:"releases"`
}

// HandleChartsDashboard returns an http.HandlerFunc for GET /api/v1/charts/dashboard.
// It returns license distribution counts and hourly heartbeat timeline for the last 24h.
// When no data is available, returns empty maps/slices (not errors) for graceful empty state handling.
func HandleChartsDashboard(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()

		// Get license distribution by status.
		licenseCounts, err := database.CountLicensesByStatusCtx(ctx)
		if err != nil {
			if IsContextTimeout(err) {
				WriteDBTimeoutResponse(w)
				return
			}
			log.Printf("charts/dashboard: license distribution error: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		// Ensure non-nil map for consistent JSON output.
		if licenseCounts == nil {
			licenseCounts = make(map[string]int)
		}

		// Get heartbeat timeline (last 24h, hourly).
		timeline, err := database.GetHeartbeatTimelineCtx(ctx)
		if err != nil {
			if IsContextTimeout(err) {
				WriteDBTimeoutResponse(w)
				return
			}
			log.Printf("charts/dashboard: heartbeat timeline error: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		writeJSON(w, http.StatusOK, dashboardChartData{
			LicenseDistribution: licenseCounts,
			HeartbeatTimeline:   timeline,
		})
	}
}

// HandleChartsReleases returns an http.HandlerFunc for GET /api/v1/charts/releases.
// It returns release version points for the release history chart.
// When no data is available, returns an empty slice (not an error) for graceful empty state handling.
func HandleChartsReleases(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()

		// Get release points for chart.
		releases, err := database.GetReleasePointsCtx(ctx)
		if err != nil {
			if IsContextTimeout(err) {
				WriteDBTimeoutResponse(w)
				return
			}
			log.Printf("charts/releases: error: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		writeJSON(w, http.StatusOK, releaseChartData{
			Releases: releases,
		})
	}
}
