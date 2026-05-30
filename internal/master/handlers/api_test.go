package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"kiro_waf/internal/master/db"
	"kiro_waf/internal/master/models"
)

// newTestDB creates an in-memory SQLite database seeded with test data.
func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	d, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("newTestDB: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// seedActiveLicense inserts an active license with the given key and returns it.
func seedActiveLicense(t *testing.T, database *db.DB, key string) *models.License {
	t.Helper()
	now := time.Now().Truncate(time.Second)
	l := &models.License{
		LicenseID:    "lic-" + key,
		LicenseKey:   key,
		CustomerID:   "cust-001",
		CustomerName: "Test Customer",
		Plan:         "community",
		Status:       "active",
		ValidDays:    365,
		CreatedAt:    now,
		ExpiresAt:    now.Add(365 * 24 * time.Hour),
	}
	if err := database.CreateLicense(l); err != nil {
		t.Fatalf("seedActiveLicense: %v", err)
	}
	return l
}

// seedExpiredLicense inserts an expired license with the given key.
func seedExpiredLicense(t *testing.T, database *db.DB, key string) *models.License {
	t.Helper()
	now := time.Now().Truncate(time.Second)
	l := &models.License{
		LicenseID:    "lic-" + key,
		LicenseKey:   key,
		CustomerID:   "cust-002",
		CustomerName: "Expired Customer",
		Plan:         "community",
		Status:       "active",
		ValidDays:    365,
		CreatedAt:    now.Add(-400 * 24 * time.Hour),
		ExpiresAt:    now.Add(-30 * 24 * time.Hour), // expired 30 days ago
	}
	if err := database.CreateLicense(l); err != nil {
		t.Fatalf("seedExpiredLicense: %v", err)
	}
	return l
}

// seedRelease inserts a release for the given component, channel, and version.
func seedRelease(t *testing.T, database *db.DB, component, channel, version string) *models.Release {
	t.Helper()
	r := &models.Release{
		Component:   component,
		Channel:     channel,
		Version:     version,
		ArtifactURL: "https://releases.example.com/" + component + "-" + version + ".tar.gz",
		SHA256:      "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		Notes:       "release " + version,
		MinVersion:  "0.0.0",
		CreatedAt:   time.Now().Truncate(time.Second),
	}
	if err := database.CreateRelease(r); err != nil {
		t.Fatalf("seedRelease: %v", err)
	}
	return r
}

// ============================================================
// Heartbeat Handler Tests
// ============================================================

func TestHandleHeartbeat_ValidLicenseKey(t *testing.T) {
	database := newTestDB(t)
	seedActiveLicense(t, database, "valid-key-001")

	body := `{"license_key":"valid-key-001","node_id":"node-1","fingerprint_hash":"fp-123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/heartbeat", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleHeartbeat(database)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp heartbeatResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Valid {
		t.Error("expected valid = true")
	}
	if resp.Lock {
		t.Error("expected lock = false")
	}
	if resp.Status != "active" {
		t.Errorf("status = %q, want %q", resp.Status, "active")
	}
	if resp.ExpiresAt == "" {
		t.Error("expected non-empty expires_at")
	}
}

func TestHandleHeartbeat_InvalidLicenseKey(t *testing.T) {
	database := newTestDB(t)
	// No license seeded — key does not exist.

	body := `{"license_key":"nonexistent-key","node_id":"node-1","fingerprint_hash":"fp-123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/heartbeat", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleHeartbeat(database)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp heartbeatResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Valid {
		t.Error("expected valid = false for invalid key")
	}
	if !resp.Lock {
		t.Error("expected lock = true for invalid key")
	}
}

func TestHandleHeartbeat_ExpiredLicense(t *testing.T) {
	database := newTestDB(t)
	seedExpiredLicense(t, database, "expired-key-001")

	body := `{"license_key":"expired-key-001","node_id":"node-1","fingerprint_hash":"fp-123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/heartbeat", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleHeartbeat(database)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp heartbeatResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Valid {
		t.Error("expected valid = false for expired license")
	}
	if resp.Lock {
		t.Error("expected lock = false for expired license (downgraded to community)")
	}
	if resp.PlanConfig == nil {
		t.Error("expected plan_config to be set for expired license (community downgrade)")
	} else if resp.PlanConfig.RPMPerIP != 60 {
		t.Errorf("expected community rpm_per_ip = 60, got %d", resp.PlanConfig.RPMPerIP)
	}
}

func TestHandleHeartbeat_InvalidJSON(t *testing.T) {
	database := newTestDB(t)

	body := `{invalid json`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/heartbeat", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleHeartbeat(database)(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleHeartbeat_MissingFields(t *testing.T) {
	database := newTestDB(t)

	tests := []struct {
		name string
		body string
	}{
		{"missing license_key", `{"node_id":"node-1"}`},
		{"missing node_id", `{"license_key":"some-key"}`},
		{"empty license_key", `{"license_key":"","node_id":"node-1"}`},
		{"empty node_id", `{"license_key":"some-key","node_id":""}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/heartbeat", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			HandleHeartbeat(database)(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestHandleHeartbeat_WrongMethod(t *testing.T) {
	database := newTestDB(t)

	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/v1/heartbeat", nil)
			w := httptest.NewRecorder()

			HandleHeartbeat(database)(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

func TestHandleHeartbeat_SuspendedLicense(t *testing.T) {
	database := newTestDB(t)
	// Seed a suspended license.
	now := time.Now().Truncate(time.Second)
	l := &models.License{
		LicenseID:    "lic-suspended-001",
		LicenseKey:   "suspended-key-001",
		CustomerID:   "cust-003",
		CustomerName: "Suspended Customer",
		Plan:         "pro",
		Status:       "suspended",
		ValidDays:    365,
		CreatedAt:    now,
		ExpiresAt:    now.Add(365 * 24 * time.Hour),
	}
	if err := database.CreateLicense(l); err != nil {
		t.Fatalf("seed suspended license: %v", err)
	}

	body := `{"license_key":"suspended-key-001","node_id":"node-1","fingerprint_hash":"fp-123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/heartbeat", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleHeartbeat(database)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp heartbeatResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Valid {
		t.Error("expected valid = false for suspended license")
	}
	if resp.Lock {
		t.Error("expected lock = false for suspended license (not hard lock)")
	}
	if resp.Status != "suspended" {
		t.Errorf("status = %q, want %q", resp.Status, "suspended")
	}
	if resp.Plan != "pro" {
		t.Errorf("plan = %q, want %q", resp.Plan, "pro")
	}
	if resp.Reason != "license_suspended" {
		t.Errorf("reason = %q, want %q", resp.Reason, "license_suspended")
	}
}

func TestHandleHeartbeat_PlanFieldIncluded(t *testing.T) {
	database := newTestDB(t)
	seedActiveLicense(t, database, "plan-test-key")

	body := `{"license_key":"plan-test-key","node_id":"node-1","fingerprint_hash":"fp-123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/heartbeat", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleHeartbeat(database)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp heartbeatResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Plan != "community" {
		t.Errorf("plan = %q, want %q", resp.Plan, "community")
	}
}

func TestHandleHeartbeat_RevokedLicense(t *testing.T) {
	database := newTestDB(t)
	now := time.Now().Truncate(time.Second)
	l := &models.License{
		LicenseID:    "lic-revoked-001",
		LicenseKey:   "revoked-key-001",
		CustomerID:   "cust-004",
		CustomerName: "Revoked Customer",
		Plan:         "pro",
		Status:       "revoked",
		ValidDays:    365,
		CreatedAt:    now,
		ExpiresAt:    now.Add(365 * 24 * time.Hour),
	}
	if err := database.CreateLicense(l); err != nil {
		t.Fatalf("seed revoked license: %v", err)
	}

	body := `{"license_key":"revoked-key-001","node_id":"node-1","fingerprint_hash":"fp-123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/heartbeat", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleHeartbeat(database)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp heartbeatResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Valid {
		t.Error("expected valid = false for revoked license")
	}
	if !resp.Lock {
		t.Error("expected lock = true for revoked license (hard lock)")
	}
	if resp.Status != "revoked" {
		t.Errorf("status = %q, want %q", resp.Status, "revoked")
	}
	if resp.Plan != "pro" {
		t.Errorf("plan = %q, want %q", resp.Plan, "pro")
	}
}

// ============================================================
// Update Check Handler Tests
// ============================================================

func TestHandleUpdateCheck_UpdateAvailable(t *testing.T) {
	database := newTestDB(t)
	seedRelease(t, database, "kiro-client-waf", "stable", "2.0.0")

	body := `{"component":"kiro-client-waf","channel":"stable","current_version":"1.0.0"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/update/check", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleUpdateCheck(database)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp updateCheckResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.UpdateAvailable {
		t.Error("expected update_available = true")
	}
	if resp.Release == nil {
		t.Fatal("expected non-nil release")
	}
	if resp.Release.Version != "2.0.0" {
		t.Errorf("version = %q, want %q", resp.Release.Version, "2.0.0")
	}
	if resp.Release.ArtifactURL == "" {
		t.Error("expected non-empty artifact_url")
	}
	if resp.Release.SHA256 == "" {
		t.Error("expected non-empty sha256")
	}
}

func TestHandleUpdateCheck_NoUpdate(t *testing.T) {
	database := newTestDB(t)
	// No releases seeded — nothing available.

	body := `{"component":"kiro-client-waf","channel":"stable","current_version":"1.0.0"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/update/check", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleUpdateCheck(database)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp updateCheckResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.UpdateAvailable {
		t.Error("expected update_available = false when no releases exist")
	}
	if resp.Release != nil {
		t.Error("expected nil release when no update available")
	}
}

func TestHandleUpdateCheck_SameVersion(t *testing.T) {
	database := newTestDB(t)
	seedRelease(t, database, "kiro-client-waf", "stable", "1.0.0")

	body := `{"component":"kiro-client-waf","channel":"stable","current_version":"1.0.0"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/update/check", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleUpdateCheck(database)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp updateCheckResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.UpdateAvailable {
		t.Error("expected update_available = false when already on latest version")
	}
}

func TestHandleUpdateCheck_InvalidJSON(t *testing.T) {
	database := newTestDB(t)

	body := `not valid json`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/update/check", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleUpdateCheck(database)(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateCheck_MissingFields(t *testing.T) {
	database := newTestDB(t)

	tests := []struct {
		name string
		body string
	}{
		{"missing component", `{"channel":"stable","current_version":"1.0.0"}`},
		{"missing channel", `{"component":"kiro-client-waf","current_version":"1.0.0"}`},
		{"empty component", `{"component":"","channel":"stable","current_version":"1.0.0"}`},
		{"empty channel", `{"component":"kiro-client-waf","channel":"","current_version":"1.0.0"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/update/check", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			HandleUpdateCheck(database)(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestHandleUpdateCheck_WrongMethod(t *testing.T) {
	database := newTestDB(t)

	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/v1/update/check", nil)
			w := httptest.NewRecorder()

			HandleUpdateCheck(database)(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

// ============================================================
// Health Check Handler Tests
// ============================================================

func TestHandleHealthz_OK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	HandleHealthz()(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp healthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("status = %q, want %q", resp.Status, "ok")
	}

	// Verify Content-Type header.
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

func TestHandleHealthz_WrongMethod(t *testing.T) {
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/healthz", nil)
			w := httptest.NewRecorder()

			HandleHealthz()(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}
