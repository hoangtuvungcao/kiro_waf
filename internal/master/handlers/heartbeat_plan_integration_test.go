package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kiro_waf/internal/master/models"
)

// =============================================================================
// Integration Test: Heartbeat Response with Plan Enforcement
// Requirements: 14.5, 14.7, 14.9, 14.10
// =============================================================================

// TestHeartbeat_CommunityPlan_ActiveLicense verifies that an active Community
// license receives correct plan_config in heartbeat response.
// Requirement 14.5: Client_Node continues operating with Community features.
func TestHeartbeat_CommunityPlan_ActiveLicense(t *testing.T) {
	database := newTestDB(t)

	now := time.Now().Truncate(time.Second)
	l := &models.License{
		LicenseID:       "lic-comm-active-001",
		LicenseKey:      "comm-active-key-001",
		CustomerID:      "cust-comm-001",
		CustomerName:    "Community Active User",
		FingerprintHash: "fp-comm-001",
		Plan:            "community",
		Status:          "active",
		ValidDays:       0, // Perpetual
		CreatedAt:       now,
		ExpiresAt:       time.Time{}, // Zero = perpetual
	}
	if err := database.CreateLicense(l); err != nil {
		t.Fatalf("seed community license: %v", err)
	}

	body := `{"license_key":"comm-active-key-001","node_id":"node-comm-1","fingerprint_hash":"fp-comm-001"}`
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

	// Community active license should be valid
	if !resp.Valid {
		t.Error("expected valid = true for active community license")
	}
	if resp.Lock {
		t.Error("expected lock = false for active community license")
	}
	if resp.Plan != "community" {
		t.Errorf("plan = %q, want %q", resp.Plan, "community")
	}
	if resp.Status != "active" {
		t.Errorf("status = %q, want %q", resp.Status, "active")
	}

	// Verify plan_config contains community limits
	if resp.PlanConfig == nil {
		t.Fatal("expected non-nil plan_config for community license")
	}
	if resp.PlanConfig.RPMPerIP != 60 {
		t.Errorf("plan_config.rpm_per_ip = %d, want 60", resp.PlanConfig.RPMPerIP)
	}
	if resp.PlanConfig.MaxDomains != 1 {
		t.Errorf("plan_config.max_domains = %d, want 1", resp.PlanConfig.MaxDomains)
	}
	if resp.PlanConfig.XDPEnabled {
		t.Error("plan_config.xdp_enabled should be false for community")
	}
	if resp.PlanConfig.OTAEnabled {
		t.Error("plan_config.ota_enabled should be false for community")
	}
}

// TestHeartbeat_ProPlan_ActiveLicense verifies Pro plan config in heartbeat.
func TestHeartbeat_ProPlan_ActiveLicense(t *testing.T) {
	database := newTestDB(t)

	now := time.Now().Truncate(time.Second)
	l := &models.License{
		LicenseID:       "lic-pro-active-001",
		LicenseKey:      "pro-active-key-001",
		CustomerID:      "cust-pro-001",
		CustomerName:    "Pro Active User",
		FingerprintHash: "fp-pro-001",
		Plan:            "pro",
		Status:          "active",
		ValidDays:       365,
		CreatedAt:       now,
		ExpiresAt:       now.Add(365 * 24 * time.Hour),
	}
	if err := database.CreateLicense(l); err != nil {
		t.Fatalf("seed pro license: %v", err)
	}

	body := `{"license_key":"pro-active-key-001","node_id":"node-pro-1","fingerprint_hash":"fp-pro-001"}`
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
		t.Error("expected valid = true for active pro license")
	}
	if resp.Plan != "pro" {
		t.Errorf("plan = %q, want %q", resp.Plan, "pro")
	}

	// Verify Pro plan config
	if resp.PlanConfig == nil {
		t.Fatal("expected non-nil plan_config for pro license")
	}
	if resp.PlanConfig.RPMPerIP != 120 {
		t.Errorf("plan_config.rpm_per_ip = %d, want 120", resp.PlanConfig.RPMPerIP)
	}
	if resp.PlanConfig.MaxDomains != 5 {
		t.Errorf("plan_config.max_domains = %d, want 5", resp.PlanConfig.MaxDomains)
	}
	if !resp.PlanConfig.XDPEnabled {
		t.Error("plan_config.xdp_enabled should be true for pro")
	}
	if !resp.PlanConfig.OTAEnabled {
		t.Error("plan_config.ota_enabled should be true for pro")
	}
}

// TestHeartbeat_SuspendedAndExpired_KeepsSuspended verifies that a license
// which is both suspended AND expired returns suspended status (not downgraded).
// Requirements: 14.7, 14.9, 14.10
func TestHeartbeat_SuspendedAndExpired_KeepsSuspended(t *testing.T) {
	database := newTestDB(t)

	now := time.Now().Truncate(time.Second)
	l := &models.License{
		LicenseID:       "lic-susp-exp-hb-001",
		LicenseKey:      "susp-exp-hb-key-001",
		CustomerID:      "cust-susp-exp-001",
		CustomerName:    "Suspended Expired HB User",
		FingerprintHash: "fp-susp-exp-hb-001",
		Plan:            "pro",
		Status:          "suspended",
		ValidDays:       365,
		CreatedAt:       now.Add(-400 * 24 * time.Hour),
		ExpiresAt:       now.Add(-30 * 24 * time.Hour), // Expired 30 days ago
	}
	if err := database.CreateLicense(l); err != nil {
		t.Fatalf("seed suspended+expired license: %v", err)
	}

	body := `{"license_key":"susp-exp-hb-key-001","node_id":"node-susp-1","fingerprint_hash":"fp-susp-exp-hb-001"}`
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

	// Suspended takes priority — client should stop traffic
	if resp.Valid {
		t.Error("expected valid = false for suspended license")
	}
	if resp.Lock {
		t.Error("expected lock = false for suspended (not hard lock)")
	}
	if resp.Status != "suspended" {
		t.Errorf("status = %q, want %q (suspended takes priority over expired)", resp.Status, "suspended")
	}
	if resp.Reason != "license_suspended" {
		t.Errorf("reason = %q, want %q", resp.Reason, "license_suspended")
	}
}

// TestHeartbeat_ExpiredPro_DowngradesToCommunityConfig verifies that an expired
// Pro license gets community plan_config in heartbeat response.
// Requirement 14.2: auto-downgrade to Community config.
func TestHeartbeat_ExpiredPro_DowngradesToCommunityConfig(t *testing.T) {
	database := newTestDB(t)

	now := time.Now().Truncate(time.Second)
	l := &models.License{
		LicenseID:       "lic-exp-pro-hb-001",
		LicenseKey:      "exp-pro-hb-key-001",
		CustomerID:      "cust-exp-pro-001",
		CustomerName:    "Expired Pro HB User",
		FingerprintHash: "fp-exp-pro-hb-001",
		Plan:            "pro",
		Status:          "active",
		ValidDays:       365,
		CreatedAt:       now.Add(-400 * 24 * time.Hour),
		ExpiresAt:       now.Add(-1 * time.Hour), // Expired 1 hour ago
	}
	if err := database.CreateLicense(l); err != nil {
		t.Fatalf("seed expired pro license: %v", err)
	}

	body := `{"license_key":"exp-pro-hb-key-001","node_id":"node-exp-1","fingerprint_hash":"fp-exp-pro-hb-001"}`
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

	// Expired Pro → downgraded to community config (not locked)
	if resp.Lock {
		t.Error("expected lock = false for expired pro (downgrade, not lock)")
	}
	if resp.Plan != "community" {
		t.Errorf("plan = %q, want %q (expired pro downgrades to community)", resp.Plan, "community")
	}

	// Verify community plan_config is returned
	if resp.PlanConfig == nil {
		t.Fatal("expected non-nil plan_config for expired pro (community downgrade)")
	}
	if resp.PlanConfig.RPMPerIP != 60 {
		t.Errorf("plan_config.rpm_per_ip = %d, want 60 (community)", resp.PlanConfig.RPMPerIP)
	}
	if resp.PlanConfig.MaxDomains != 1 {
		t.Errorf("plan_config.max_domains = %d, want 1 (community)", resp.PlanConfig.MaxDomains)
	}
	if resp.PlanConfig.XDPEnabled {
		t.Error("plan_config.xdp_enabled should be false (community)")
	}
	if resp.PlanConfig.OTAEnabled {
		t.Error("plan_config.ota_enabled should be false (community)")
	}
}

// TestHeartbeat_ExpiredEnterprise_DowngradesToCommunityConfig verifies Enterprise
// expired license also gets community config.
func TestHeartbeat_ExpiredEnterprise_DowngradesToCommunityConfig(t *testing.T) {
	database := newTestDB(t)

	now := time.Now().Truncate(time.Second)
	l := &models.License{
		LicenseID:       "lic-exp-ent-hb-001",
		LicenseKey:      "exp-ent-hb-key-001",
		CustomerID:      "cust-exp-ent-001",
		CustomerName:    "Expired Enterprise HB User",
		FingerprintHash: "fp-exp-ent-hb-001",
		Plan:            "enterprise",
		Status:          "active",
		ValidDays:       3650,
		CreatedAt:       now.Add(-4000 * 24 * time.Hour),
		ExpiresAt:       now.Add(-2 * time.Hour), // Expired 2 hours ago
	}
	if err := database.CreateLicense(l); err != nil {
		t.Fatalf("seed expired enterprise license: %v", err)
	}

	body := `{"license_key":"exp-ent-hb-key-001","node_id":"node-exp-ent-1","fingerprint_hash":"fp-exp-ent-hb-001"}`
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

	// Expired Enterprise → community config
	if resp.Plan != "community" {
		t.Errorf("plan = %q, want %q", resp.Plan, "community")
	}
	if resp.PlanConfig == nil {
		t.Fatal("expected non-nil plan_config")
	}
	if resp.PlanConfig.RPMPerIP != 60 {
		t.Errorf("plan_config.rpm_per_ip = %d, want 60", resp.PlanConfig.RPMPerIP)
	}
	if resp.PlanConfig.XDPEnabled {
		t.Error("plan_config.xdp_enabled should be false for community downgrade")
	}
}
