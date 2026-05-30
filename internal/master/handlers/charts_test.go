package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kiro_waf/internal/master/db"
	"kiro_waf/internal/master/models"
)

// ============================================================
// Charts Dashboard Handler Tests
// ============================================================

func TestHandleChartsDashboard_EmptyData(t *testing.T) {
	database := newTestDB(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/charts/dashboard", nil)
	w := httptest.NewRecorder()

	HandleChartsDashboard(database)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp dashboardChartData
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Empty state: should return empty map and empty slice, not nil or error.
	if resp.LicenseDistribution == nil {
		t.Error("expected non-nil license_distribution map")
	}
	if len(resp.LicenseDistribution) != 0 {
		t.Errorf("expected empty license_distribution, got %v", resp.LicenseDistribution)
	}
	if resp.HeartbeatTimeline == nil {
		t.Error("expected non-nil heartbeat_timeline slice")
	}
	if len(resp.HeartbeatTimeline) != 0 {
		t.Errorf("expected empty heartbeat_timeline, got %v", resp.HeartbeatTimeline)
	}
}

func TestHandleChartsDashboard_WithData(t *testing.T) {
	database := newTestDB(t)

	// Seed licenses with different statuses.
	seedLicenseWithStatus(t, database, "key-active-1", "active")
	seedLicenseWithStatus(t, database, "key-active-2", "active")
	seedLicenseWithStatus(t, database, "key-suspended-1", "suspended")
	seedLicenseWithStatus(t, database, "key-revoked-1", "revoked")

	// Seed heartbeats within the last 24 hours.
	now := time.Now().UTC()
	seedHeartbeat(t, database, "lic-key-active-1", "node-1", now.Add(-1*time.Hour))
	seedHeartbeat(t, database, "lic-key-active-1", "node-1", now.Add(-1*time.Hour+10*time.Minute))
	seedHeartbeat(t, database, "lic-key-active-2", "node-2", now.Add(-2*time.Hour))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/charts/dashboard", nil)
	w := httptest.NewRecorder()

	HandleChartsDashboard(database)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp dashboardChartData
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Verify license distribution.
	if resp.LicenseDistribution["active"] != 2 {
		t.Errorf("active count = %d, want 2", resp.LicenseDistribution["active"])
	}
	if resp.LicenseDistribution["suspended"] != 1 {
		t.Errorf("suspended count = %d, want 1", resp.LicenseDistribution["suspended"])
	}
	if resp.LicenseDistribution["revoked"] != 1 {
		t.Errorf("revoked count = %d, want 1", resp.LicenseDistribution["revoked"])
	}

	// Verify heartbeat timeline has entries.
	if len(resp.HeartbeatTimeline) == 0 {
		t.Error("expected non-empty heartbeat_timeline")
	}

	// Verify each entry has valid hour and positive count.
	for _, entry := range resp.HeartbeatTimeline {
		if entry.Hour == "" {
			t.Error("expected non-empty hour in timeline entry")
		}
		if entry.Count <= 0 {
			t.Errorf("expected positive count, got %d", entry.Count)
		}
	}
}

func TestHandleChartsDashboard_WrongMethod(t *testing.T) {
	database := newTestDB(t)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/v1/charts/dashboard", nil)
			w := httptest.NewRecorder()

			HandleChartsDashboard(database)(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

func TestHandleChartsDashboard_ContentType(t *testing.T) {
	database := newTestDB(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/charts/dashboard", nil)
	w := httptest.NewRecorder()

	HandleChartsDashboard(database)(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

// ============================================================
// Charts Releases Handler Tests
// ============================================================

func TestHandleChartsReleases_EmptyData(t *testing.T) {
	database := newTestDB(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/charts/releases", nil)
	w := httptest.NewRecorder()

	HandleChartsReleases(database)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp releaseChartData
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Empty state: should return empty slice, not nil or error.
	if resp.Releases == nil {
		t.Error("expected non-nil releases slice")
	}
	if len(resp.Releases) != 0 {
		t.Errorf("expected empty releases, got %v", resp.Releases)
	}
}

func TestHandleChartsReleases_WithData(t *testing.T) {
	database := newTestDB(t)

	// Seed releases.
	seedRelease(t, database, "kiro-client-waf", "stable", "1.0.0")
	seedRelease(t, database, "kiro-client-waf", "stable", "1.1.0")
	seedRelease(t, database, "kiro-client-waf", "stable", "2.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/charts/releases", nil)
	w := httptest.NewRecorder()

	HandleChartsReleases(database)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp releaseChartData
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Releases) != 3 {
		t.Fatalf("releases count = %d, want 3", len(resp.Releases))
	}

	// Verify each release has version and created_at.
	for _, r := range resp.Releases {
		if r.Version == "" {
			t.Error("expected non-empty version")
		}
		if r.CreatedAt == "" {
			t.Error("expected non-empty created_at")
		}
	}

	// Verify versions are present.
	versions := make(map[string]bool)
	for _, r := range resp.Releases {
		versions[r.Version] = true
	}
	if !versions["1.0.0"] || !versions["1.1.0"] || !versions["2.0.0"] {
		t.Errorf("expected all seeded versions, got %v", versions)
	}
}

func TestHandleChartsReleases_WrongMethod(t *testing.T) {
	database := newTestDB(t)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/v1/charts/releases", nil)
			w := httptest.NewRecorder()

			HandleChartsReleases(database)(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

func TestHandleChartsReleases_ContentType(t *testing.T) {
	database := newTestDB(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/charts/releases", nil)
	w := httptest.NewRecorder()

	HandleChartsReleases(database)(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

// ============================================================
// Test Helpers
// ============================================================

// seedLicenseWithStatus inserts a license with the given key and status.
func seedLicenseWithStatus(t *testing.T, database *db.DB, key, status string) *models.License {
	t.Helper()
	now := time.Now().Truncate(time.Second)
	l := &models.License{
		LicenseID:    "lic-" + key,
		LicenseKey:   key,
		CustomerID:   "cust-001",
		CustomerName: "Test Customer",
		Plan:         "community",
		Status:       status,
		ValidDays:    365,
		CreatedAt:    now,
		ExpiresAt:    now.Add(365 * 24 * time.Hour),
	}
	if err := database.CreateLicense(l); err != nil {
		t.Fatalf("seedLicenseWithStatus: %v", err)
	}
	return l
}

// seedHeartbeat inserts a heartbeat record at the given time.
func seedHeartbeat(t *testing.T, database *db.DB, licenseID, nodeID string, createdAt time.Time) {
	t.Helper()
	hb := &models.Heartbeat{
		LicenseID:       licenseID,
		NodeID:          nodeID,
		ClientIP:        "192.168.1.1",
		FingerprintHash: "fp-test",
		Stats:           map[string]any{"test": true},
		CreatedAt:       createdAt,
	}
	if err := database.LogHeartbeat(hb); err != nil {
		t.Fatalf("seedHeartbeat: %v", err)
	}
}
