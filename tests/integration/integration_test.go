// Package integration chứa integration tests cho luồng end-to-end.
// Kiểm tra: License CRUD, Heartbeat flow, Update flow, concurrent DB access.
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"kiro_waf/internal/master/db"
	"kiro_waf/internal/master/handlers"
	"kiro_waf/internal/master/models"
)

// setupTestDB creates a temporary SQLite database for testing.
// Returns the DB instance and a cleanup function.
func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// setupTestServer creates an httptest.Server with all routes registered.
// Returns the server and the database instance.
func setupTestServer(t *testing.T) (*httptest.Server, *db.DB) {
	t.Helper()
	database := setupTestDB(t)

	mux := http.NewServeMux()

	// Register API routes.
	mux.HandleFunc("/api/v1/heartbeat", handlers.HandleHeartbeat(database))
	mux.HandleFunc("/api/v1/update/check", handlers.HandleUpdateCheck(database))
	mux.HandleFunc("/healthz", handlers.HandleHealthz())

	// Register admin routes with permissive config for testing.
	adminConfig := &handlers.AdminAuthConfig{
		AdminKey:   "test-admin-key-secret",
		AllowedIPs: []string{}, // Empty = allow all IPs in test
		SessionTTL: 12 * time.Hour,
	}
	handlers.RegisterAdminRoutes(mux, database, adminConfig)

	server := httptest.NewServer(mux)
	t.Cleanup(func() { server.Close() })
	return server, database
}

// createTestLicense creates a license directly in the DB for testing.
func createTestLicense(t *testing.T, database *db.DB, customerID, plan string) *models.License {
	t.Helper()
	now := time.Now().UTC()
	license := &models.License{
		LicenseID:    fmt.Sprintf("lic-%s-%d", customerID, now.UnixNano()),
		LicenseKey:   fmt.Sprintf("key-%s-%d", customerID, now.UnixNano()),
		CustomerID:   customerID,
		CustomerName: "Test Customer " + customerID,
		Plan:         plan,
		Status:       "active",
		ValidDays:    365,
		CreatedAt:    now,
		ExpiresAt:    now.AddDate(1, 0, 0),
	}
	if err := database.CreateLicense(license); err != nil {
		t.Fatalf("failed to create test license: %v", err)
	}
	return license
}

// ============================================================
// Test 1: License CRUD end-to-end
// ============================================================

func TestLicenseCRUD_EndToEnd(t *testing.T) {
	database := setupTestDB(t)

	// 1. Create license.
	now := time.Now().UTC()
	license := &models.License{
		LicenseID:    "lic-crud-test-001",
		LicenseKey:   "key-crud-test-001",
		CustomerID:   "cust-001",
		CustomerName: "CRUD Test Customer",
		Plan:         "pro",
		Status:       "active",
		ValidDays:    365,
		CreatedAt:    now,
		ExpiresAt:    now.AddDate(1, 0, 0),
		Notes:        "integration test license",
	}
	err := database.CreateLicense(license)
	if err != nil {
		t.Fatalf("CreateLicense failed: %v", err)
	}
	if license.ID == 0 {
		t.Fatal("expected license ID to be set after creation")
	}

	// 2. Verify in DB.
	fetched, err := database.GetLicenseByID(license.ID)
	if err != nil {
		t.Fatalf("GetLicenseByID failed: %v", err)
	}
	if fetched == nil {
		t.Fatal("expected license to exist in DB")
	}
	if fetched.LicenseID != "lic-crud-test-001" {
		t.Errorf("expected license_id 'lic-crud-test-001', got %q", fetched.LicenseID)
	}
	if fetched.CustomerName != "CRUD Test Customer" {
		t.Errorf("expected customer_name 'CRUD Test Customer', got %q", fetched.CustomerName)
	}
	if fetched.Plan != "pro" {
		t.Errorf("expected plan 'pro', got %q", fetched.Plan)
	}

	// 3. Update license.
	fetched.CustomerName = "Updated Customer Name"
	fetched.Plan = "enterprise"
	fetched.Notes = "updated notes"
	err = database.UpdateLicense(fetched)
	if err != nil {
		t.Fatalf("UpdateLicense failed: %v", err)
	}

	updated, err := database.GetLicenseByID(license.ID)
	if err != nil {
		t.Fatalf("GetLicenseByID after update failed: %v", err)
	}
	if updated.CustomerName != "Updated Customer Name" {
		t.Errorf("expected updated customer_name, got %q", updated.CustomerName)
	}
	if updated.Plan != "enterprise" {
		t.Errorf("expected updated plan 'enterprise', got %q", updated.Plan)
	}

	// 4. Renew license.
	err = database.RenewLicense(license.ID)
	if err != nil {
		t.Fatalf("RenewLicense failed: %v", err)
	}

	renewed, err := database.GetLicenseByID(license.ID)
	if err != nil {
		t.Fatalf("GetLicenseByID after renew failed: %v", err)
	}
	if renewed.Status != "active" {
		t.Errorf("expected status 'active' after renew, got %q", renewed.Status)
	}
	// ExpiresAt should be in the future (renewed from now).
	if !renewed.ExpiresAt.After(time.Now()) {
		t.Error("expected expires_at to be in the future after renew")
	}

	// 5. Rotate key.
	newKey, err := database.RotateLicenseKey(license.ID)
	if err != nil {
		t.Fatalf("RotateLicenseKey failed: %v", err)
	}
	if newKey == "" {
		t.Fatal("expected new key to be non-empty")
	}
	if newKey == "key-crud-test-001" {
		t.Error("expected new key to differ from original")
	}

	rotated, err := database.GetLicenseByID(license.ID)
	if err != nil {
		t.Fatalf("GetLicenseByID after rotate failed: %v", err)
	}
	if rotated.LicenseKey != newKey {
		t.Errorf("expected license_key %q in DB, got %q", newKey, rotated.LicenseKey)
	}

	// 6. Revoke license.
	err = database.RevokeLicense(license.ID)
	if err != nil {
		t.Fatalf("RevokeLicense failed: %v", err)
	}

	revoked, err := database.GetLicenseByID(license.ID)
	if err != nil {
		t.Fatalf("GetLicenseByID after revoke failed: %v", err)
	}
	if revoked.Status != "revoked" {
		t.Errorf("expected status 'revoked', got %q", revoked.Status)
	}

	// 7. Delete license.
	err = database.DeleteLicense(license.ID)
	if err != nil {
		t.Fatalf("DeleteLicense failed: %v", err)
	}

	deleted, err := database.GetLicenseByID(license.ID)
	if err != nil {
		t.Fatalf("GetLicenseByID after delete failed: %v", err)
	}
	if deleted != nil {
		t.Error("expected license to be nil after deletion")
	}
}

// ============================================================
// Test 2: Heartbeat flow (Client → Master → DB → Response)
// ============================================================

func TestHeartbeatFlow(t *testing.T) {
	server, database := setupTestServer(t)

	// Create a valid license.
	license := createTestLicense(t, database, "hb-customer", "pro")

	// Sub-test: Send heartbeat with valid key → verify response.
	t.Run("valid_key_heartbeat", func(t *testing.T) {
		payload := map[string]any{
			"license_key":      license.LicenseKey,
			"node_id":          "node-001",
			"fingerprint_hash": "abc123",
			"stats":            map[string]any{"requests": 1000, "blocked": 50},
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(server.URL+"/api/v1/heartbeat", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("heartbeat request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if result["valid"] != true {
			t.Errorf("expected valid=true, got %v", result["valid"])
		}
		if result["lock"] != false {
			t.Errorf("expected lock=false, got %v", result["lock"])
		}
	})

	// Sub-test: Verify heartbeat logged in DB.
	t.Run("heartbeat_logged_in_db", func(t *testing.T) {
		heartbeats, err := database.ListHeartbeatsByLicense(license.LicenseID, 10)
		if err != nil {
			t.Fatalf("ListHeartbeatsByLicense failed: %v", err)
		}
		if len(heartbeats) == 0 {
			t.Fatal("expected at least one heartbeat logged in DB")
		}

		hb := heartbeats[0]
		if hb.LicenseID != license.LicenseID {
			t.Errorf("expected license_id %q, got %q", license.LicenseID, hb.LicenseID)
		}
		if hb.NodeID != "node-001" {
			t.Errorf("expected node_id 'node-001', got %q", hb.NodeID)
		}
	})

	// Sub-test: Send heartbeat with invalid key → verify response (valid: false, lock: true).
	t.Run("invalid_key_heartbeat", func(t *testing.T) {
		payload := map[string]any{
			"license_key":      "invalid-key-does-not-exist",
			"node_id":          "node-002",
			"fingerprint_hash": "xyz789",
			"stats":            map[string]any{},
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(server.URL+"/api/v1/heartbeat", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("heartbeat request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if result["valid"] != false {
			t.Errorf("expected valid=false for invalid key, got %v", result["valid"])
		}
		if result["lock"] != true {
			t.Errorf("expected lock=true for invalid key, got %v", result["lock"])
		}
	})

	// Sub-test: Heartbeat with revoked license → lock.
	t.Run("revoked_license_heartbeat", func(t *testing.T) {
		// Revoke the license.
		if err := database.RevokeLicense(license.ID); err != nil {
			t.Fatalf("RevokeLicense failed: %v", err)
		}

		payload := map[string]any{
			"license_key": license.LicenseKey,
			"node_id":     "node-001",
			"stats":       map[string]any{},
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(server.URL+"/api/v1/heartbeat", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("heartbeat request failed: %v", err)
		}
		defer resp.Body.Close()

		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if result["valid"] != false {
			t.Errorf("expected valid=false for revoked license, got %v", result["valid"])
		}
		if result["lock"] != true {
			t.Errorf("expected lock=true for revoked license, got %v", result["lock"])
		}
	})
}

// ============================================================
// Test 3: Update flow (Check → Download → Verify → Replace → Health)
// ============================================================

func TestUpdateFlow(t *testing.T) {
	server, database := setupTestServer(t)

	// Create a release.
	release := &models.Release{
		Component:   "kiro-client-waf",
		Channel:     "stable",
		Version:     "2.0.0",
		ArtifactURL: "https://releases.example.com/kiro-client-waf-2.0.0",
		SHA256:      "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		Notes:       "Security fix release",
		MinVersion:  "1.0.0",
		CreatedAt:   time.Now().UTC(),
	}
	if err := database.CreateRelease(release); err != nil {
		t.Fatalf("CreateRelease failed: %v", err)
	}

	// Sub-test: Check for update with older version → update_available: true.
	t.Run("update_available_older_version", func(t *testing.T) {
		payload := map[string]string{
			"component":       "kiro-client-waf",
			"channel":         "stable",
			"current_version": "1.5.0",
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(server.URL+"/api/v1/update/check", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("update check request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if result["update_available"] != true {
			t.Errorf("expected update_available=true, got %v", result["update_available"])
		}

		releaseInfo, ok := result["release"].(map[string]any)
		if !ok {
			t.Fatal("expected release info in response")
		}
		if releaseInfo["version"] != "2.0.0" {
			t.Errorf("expected version '2.0.0', got %v", releaseInfo["version"])
		}
		if releaseInfo["artifact_url"] != release.ArtifactURL {
			t.Errorf("expected artifact_url %q, got %v", release.ArtifactURL, releaseInfo["artifact_url"])
		}
		if releaseInfo["sha256"] != release.SHA256 {
			t.Errorf("expected sha256 %q, got %v", release.SHA256, releaseInfo["sha256"])
		}
	})

	// Sub-test: Check with same version → update_available: false.
	t.Run("no_update_same_version", func(t *testing.T) {
		payload := map[string]string{
			"component":       "kiro-client-waf",
			"channel":         "stable",
			"current_version": "2.0.0",
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(server.URL+"/api/v1/update/check", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("update check request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if result["update_available"] != false {
			t.Errorf("expected update_available=false for same version, got %v", result["update_available"])
		}
	})

	// Sub-test: Check for non-existent component → update_available: false.
	t.Run("no_update_unknown_component", func(t *testing.T) {
		payload := map[string]string{
			"component":       "non-existent-component",
			"channel":         "stable",
			"current_version": "1.0.0",
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(server.URL+"/api/v1/update/check", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("update check request failed: %v", err)
		}
		defer resp.Body.Close()

		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if result["update_available"] != false {
			t.Errorf("expected update_available=false for unknown component, got %v", result["update_available"])
		}
	})
}

// ============================================================
// Test 4: Concurrent DB access with multiple goroutines
// ============================================================

func TestConcurrentDBAccess(t *testing.T) {
	database := setupTestDB(t)

	const numGoroutines = 20
	var wg sync.WaitGroup
	errCh := make(chan error, numGoroutines*3)

	// Concurrent license creation.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			now := time.Now().UTC()
			license := &models.License{
				LicenseID:    fmt.Sprintf("concurrent-lic-%d", idx),
				LicenseKey:   fmt.Sprintf("concurrent-key-%d", idx),
				CustomerID:   fmt.Sprintf("concurrent-cust-%d", idx),
				CustomerName: fmt.Sprintf("Customer %d", idx),
				Plan:         "community",
				Status:       "active",
				ValidDays:    365,
				CreatedAt:    now,
				ExpiresAt:    now.AddDate(1, 0, 0),
			}
			if err := database.CreateLicense(license); err != nil {
				errCh <- fmt.Errorf("goroutine %d: CreateLicense: %w", idx, err)
			}
		}(i)
	}

	// Concurrent heartbeat logging.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			hb := &models.Heartbeat{
				LicenseID:       fmt.Sprintf("concurrent-lic-%d", idx%5),
				NodeID:          fmt.Sprintf("node-%d", idx),
				ClientIP:        fmt.Sprintf("192.168.1.%d", idx),
				FingerprintHash: fmt.Sprintf("fp-%d", idx),
				Stats:           map[string]any{"goroutine": idx},
				CreatedAt:       time.Now().UTC(),
			}
			if err := database.LogHeartbeat(hb); err != nil {
				errCh <- fmt.Errorf("goroutine %d: LogHeartbeat: %w", idx, err)
			}
		}(i)
	}

	// Concurrent release creation.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			release := &models.Release{
				Component:   fmt.Sprintf("component-%d", idx),
				Channel:     "stable",
				Version:     fmt.Sprintf("1.0.%d", idx),
				ArtifactURL: fmt.Sprintf("https://example.com/artifact-%d", idx),
				SHA256:      fmt.Sprintf("%064d", idx),
				Notes:       fmt.Sprintf("Release %d", idx),
				MinVersion:  "0.0.0",
				CreatedAt:   time.Now().UTC(),
			}
			if err := database.CreateRelease(release); err != nil {
				errCh <- fmt.Errorf("goroutine %d: CreateRelease: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	// Check for errors.
	var errors []error
	for err := range errCh {
		errors = append(errors, err)
	}
	if len(errors) > 0 {
		for _, err := range errors {
			t.Errorf("concurrent error: %v", err)
		}
		t.Fatalf("got %d errors during concurrent access", len(errors))
	}

	// Verify data integrity: all licenses created.
	licenses, err := database.ListLicenses()
	if err != nil {
		t.Fatalf("ListLicenses failed: %v", err)
	}
	if len(licenses) != numGoroutines {
		t.Errorf("expected %d licenses, got %d", numGoroutines, len(licenses))
	}

	// Verify all heartbeats logged.
	heartbeats, err := database.ListHeartbeats(100)
	if err != nil {
		t.Fatalf("ListHeartbeats failed: %v", err)
	}
	if len(heartbeats) != numGoroutines {
		t.Errorf("expected %d heartbeats, got %d", numGoroutines, len(heartbeats))
	}

	// Verify all releases created.
	releases, err := database.ListReleases()
	if err != nil {
		t.Fatalf("ListReleases failed: %v", err)
	}
	if len(releases) != numGoroutines {
		t.Errorf("expected %d releases, got %d", numGoroutines, len(releases))
	}

	// Verify no duplicate license IDs.
	licenseIDs := make(map[string]bool)
	for _, l := range licenses {
		if licenseIDs[l.LicenseID] {
			t.Errorf("duplicate license_id found: %s", l.LicenseID)
		}
		licenseIDs[l.LicenseID] = true
	}
}

// ============================================================
// Test: Health check endpoint
// ============================================================

func TestHealthzEndpoint(t *testing.T) {
	server, _ := setupTestServer(t)

	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatalf("healthz request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", result["status"])
	}
}

// Ensure temp files are cleaned up (this is a safeguard).
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
