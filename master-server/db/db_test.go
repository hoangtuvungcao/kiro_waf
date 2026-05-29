package db

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"kiro_waf/master-server/models"
)

// newTestDB creates a temporary SQLite database for testing.
// Uses a temp file (not :memory:) to properly test WAL mode.
func newTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	d, err := New(dbPath)
	if err != nil {
		t.Fatalf("newTestDB: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// newInMemoryDB creates an in-memory SQLite database for fast tests.
func newInMemoryDB(t *testing.T) *DB {
	t.Helper()
	d, err := New(":memory:")
	if err != nil {
		t.Fatalf("newInMemoryDB: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func sampleLicense() *models.License {
	now := time.Now().Truncate(time.Second)
	return &models.License{
		LicenseID:       "lic-001",
		LicenseKey:      "key-abc-123",
		CustomerID:      "cust-001",
		CustomerName:    "Test Customer",
		ClientIP:        "192.168.1.100",
		FingerprintHash: "fp-hash-001",
		Plan:            "community",
		Status:          "active",
		ValidDays:       365,
		CreatedAt:       now,
		ExpiresAt:       now.Add(365 * 24 * time.Hour),
		Notes:           "test license",
	}
}


// ============================================================
// License CRUD Tests
// ============================================================

func TestCreateLicense(t *testing.T) {
	d := newInMemoryDB(t)
	l := sampleLicense()

	if err := d.CreateLicense(l); err != nil {
		t.Fatalf("CreateLicense: %v", err)
	}
	if l.ID == 0 {
		t.Fatal("expected non-zero ID after create")
	}
}

func TestCreateLicense_DuplicateKey(t *testing.T) {
	d := newInMemoryDB(t)
	l1 := sampleLicense()
	if err := d.CreateLicense(l1); err != nil {
		t.Fatalf("CreateLicense first: %v", err)
	}

	l2 := sampleLicense()
	l2.LicenseID = "lic-002" // different license_id but same key
	err := d.CreateLicense(l2)
	if err == nil {
		t.Fatal("expected error for duplicate license_key")
	}
}

func TestCreateLicense_DuplicateLicenseID(t *testing.T) {
	d := newInMemoryDB(t)
	l1 := sampleLicense()
	if err := d.CreateLicense(l1); err != nil {
		t.Fatalf("CreateLicense first: %v", err)
	}

	l2 := sampleLicense()
	l2.LicenseKey = "key-different" // different key but same license_id
	err := d.CreateLicense(l2)
	if err == nil {
		t.Fatal("expected error for duplicate license_id")
	}
}

func TestGetLicenseByID(t *testing.T) {
	d := newInMemoryDB(t)
	l := sampleLicense()
	if err := d.CreateLicense(l); err != nil {
		t.Fatalf("CreateLicense: %v", err)
	}

	got, err := d.GetLicenseByID(l.ID)
	if err != nil {
		t.Fatalf("GetLicenseByID: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil license")
	}
	if got.LicenseID != l.LicenseID {
		t.Errorf("LicenseID = %q, want %q", got.LicenseID, l.LicenseID)
	}
	if got.CustomerName != l.CustomerName {
		t.Errorf("CustomerName = %q, want %q", got.CustomerName, l.CustomerName)
	}
	if got.Plan != l.Plan {
		t.Errorf("Plan = %q, want %q", got.Plan, l.Plan)
	}
	if got.Status != l.Status {
		t.Errorf("Status = %q, want %q", got.Status, l.Status)
	}
}

func TestGetLicenseByID_NotFound(t *testing.T) {
	d := newInMemoryDB(t)
	got, err := d.GetLicenseByID(9999)
	if err != nil {
		t.Fatalf("GetLicenseByID: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for non-existent ID")
	}
}

func TestGetLicenseByKey(t *testing.T) {
	d := newInMemoryDB(t)
	l := sampleLicense()
	if err := d.CreateLicense(l); err != nil {
		t.Fatalf("CreateLicense: %v", err)
	}

	got, err := d.GetLicenseByKey(l.LicenseKey)
	if err != nil {
		t.Fatalf("GetLicenseByKey: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil license")
	}
	if got.ID != l.ID {
		t.Errorf("ID = %d, want %d", got.ID, l.ID)
	}
}

func TestGetLicenseByKey_NotFound(t *testing.T) {
	d := newInMemoryDB(t)
	got, err := d.GetLicenseByKey("nonexistent-key")
	if err != nil {
		t.Fatalf("GetLicenseByKey: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for non-existent key")
	}
}

func TestGetLicenseByLicenseID(t *testing.T) {
	d := newInMemoryDB(t)
	l := sampleLicense()
	if err := d.CreateLicense(l); err != nil {
		t.Fatalf("CreateLicense: %v", err)
	}

	got, err := d.GetLicenseByLicenseID(l.LicenseID)
	if err != nil {
		t.Fatalf("GetLicenseByLicenseID: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil license")
	}
	if got.LicenseKey != l.LicenseKey {
		t.Errorf("LicenseKey = %q, want %q", got.LicenseKey, l.LicenseKey)
	}
}

func TestGetLicenseByLicenseID_NotFound(t *testing.T) {
	d := newInMemoryDB(t)
	got, err := d.GetLicenseByLicenseID("nonexistent-lic-id")
	if err != nil {
		t.Fatalf("GetLicenseByLicenseID: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for non-existent license_id")
	}
}


func TestListLicenses(t *testing.T) {
	d := newInMemoryDB(t)

	// Empty list
	list, err := d.ListLicenses()
	if err != nil {
		t.Fatalf("ListLicenses empty: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 licenses, got %d", len(list))
	}

	// Add two licenses
	l1 := sampleLicense()
	l1.LicenseID = "lic-001"
	l1.LicenseKey = "key-001"
	if err := d.CreateLicense(l1); err != nil {
		t.Fatalf("CreateLicense l1: %v", err)
	}

	l2 := sampleLicense()
	l2.LicenseID = "lic-002"
	l2.LicenseKey = "key-002"
	l2.CreatedAt = l1.CreatedAt.Add(time.Second)
	if err := d.CreateLicense(l2); err != nil {
		t.Fatalf("CreateLicense l2: %v", err)
	}

	list, err = d.ListLicenses()
	if err != nil {
		t.Fatalf("ListLicenses: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 licenses, got %d", len(list))
	}
	// Ordered by created_at DESC, so l2 first
	if list[0].LicenseID != "lic-002" {
		t.Errorf("first license = %q, want lic-002", list[0].LicenseID)
	}
}

func TestUpdateLicense(t *testing.T) {
	d := newInMemoryDB(t)
	l := sampleLicense()
	if err := d.CreateLicense(l); err != nil {
		t.Fatalf("CreateLicense: %v", err)
	}

	l.CustomerName = "Updated Customer"
	l.Plan = "enterprise"
	l.Notes = "updated notes"
	if err := d.UpdateLicense(l); err != nil {
		t.Fatalf("UpdateLicense: %v", err)
	}

	got, err := d.GetLicenseByID(l.ID)
	if err != nil {
		t.Fatalf("GetLicenseByID: %v", err)
	}
	if got.CustomerName != "Updated Customer" {
		t.Errorf("CustomerName = %q, want %q", got.CustomerName, "Updated Customer")
	}
	if got.Plan != "enterprise" {
		t.Errorf("Plan = %q, want %q", got.Plan, "enterprise")
	}
	if got.Notes != "updated notes" {
		t.Errorf("Notes = %q, want %q", got.Notes, "updated notes")
	}
}

func TestUpdateLicense_NotFound(t *testing.T) {
	d := newInMemoryDB(t)
	l := sampleLicense()
	l.ID = 9999
	err := d.UpdateLicense(l)
	if err == nil {
		t.Fatal("expected error for non-existent license")
	}
}

func TestDeleteLicense(t *testing.T) {
	d := newInMemoryDB(t)
	l := sampleLicense()
	if err := d.CreateLicense(l); err != nil {
		t.Fatalf("CreateLicense: %v", err)
	}

	if err := d.DeleteLicense(l.ID); err != nil {
		t.Fatalf("DeleteLicense: %v", err)
	}

	got, err := d.GetLicenseByID(l.ID)
	if err != nil {
		t.Fatalf("GetLicenseByID after delete: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestDeleteLicense_NotFound(t *testing.T) {
	d := newInMemoryDB(t)
	err := d.DeleteLicense(9999)
	if err == nil {
		t.Fatal("expected error for non-existent license")
	}
}

func TestRenewLicense(t *testing.T) {
	d := newInMemoryDB(t)
	l := sampleLicense()
	l.Status = "expired"
	if err := d.CreateLicense(l); err != nil {
		t.Fatalf("CreateLicense: %v", err)
	}

	if err := d.RenewLicense(l.ID); err != nil {
		t.Fatalf("RenewLicense: %v", err)
	}

	got, err := d.GetLicenseByID(l.ID)
	if err != nil {
		t.Fatalf("GetLicenseByID: %v", err)
	}
	if got.Status != "active" {
		t.Errorf("Status = %q, want %q", got.Status, "active")
	}
	// ExpiresAt should be in the future (at least valid_days from now minus some tolerance)
	if got.ExpiresAt.Before(time.Now().Add(364 * 24 * time.Hour)) {
		t.Errorf("ExpiresAt = %v, expected at least 364 days from now", got.ExpiresAt)
	}
}

func TestRenewLicense_NotFound(t *testing.T) {
	d := newInMemoryDB(t)
	err := d.RenewLicense(9999)
	if err == nil {
		t.Fatal("expected error for non-existent license")
	}
}

func TestRotateLicenseKey(t *testing.T) {
	d := newInMemoryDB(t)
	l := sampleLicense()
	if err := d.CreateLicense(l); err != nil {
		t.Fatalf("CreateLicense: %v", err)
	}

	oldKey := l.LicenseKey
	newKey, err := d.RotateLicenseKey(l.ID)
	if err != nil {
		t.Fatalf("RotateLicenseKey: %v", err)
	}
	if newKey == "" {
		t.Fatal("expected non-empty new key")
	}
	if newKey == oldKey {
		t.Fatal("expected new key to differ from old key")
	}

	// Verify the key was actually updated in DB
	got, err := d.GetLicenseByID(l.ID)
	if err != nil {
		t.Fatalf("GetLicenseByID: %v", err)
	}
	if got.LicenseKey != newKey {
		t.Errorf("LicenseKey = %q, want %q", got.LicenseKey, newKey)
	}

	// Old key should no longer find the license
	old, err := d.GetLicenseByKey(oldKey)
	if err != nil {
		t.Fatalf("GetLicenseByKey old: %v", err)
	}
	if old != nil {
		t.Fatal("expected nil for old key after rotation")
	}
}

func TestRotateLicenseKey_NotFound(t *testing.T) {
	d := newInMemoryDB(t)
	_, err := d.RotateLicenseKey(9999)
	if err == nil {
		t.Fatal("expected error for non-existent license")
	}
}

func TestRevokeLicense(t *testing.T) {
	d := newInMemoryDB(t)
	l := sampleLicense()
	if err := d.CreateLicense(l); err != nil {
		t.Fatalf("CreateLicense: %v", err)
	}

	if err := d.RevokeLicense(l.ID); err != nil {
		t.Fatalf("RevokeLicense: %v", err)
	}

	got, err := d.GetLicenseByID(l.ID)
	if err != nil {
		t.Fatalf("GetLicenseByID: %v", err)
	}
	if got.Status != "revoked" {
		t.Errorf("Status = %q, want %q", got.Status, "revoked")
	}
}

func TestRevokeLicense_NotFound(t *testing.T) {
	d := newInMemoryDB(t)
	err := d.RevokeLicense(9999)
	if err == nil {
		t.Fatal("expected error for non-existent license")
	}
}

func TestUpdateLicenseHeartbeat(t *testing.T) {
	d := newInMemoryDB(t)
	l := sampleLicense()
	if err := d.CreateLicense(l); err != nil {
		t.Fatalf("CreateLicense: %v", err)
	}

	// Use a time formatted as time.DateTime to match how GetLicenseByID parses it.
	// The code parses last_heartbeat_at with time.Parse(time.DateTime, ...) which expects "2006-01-02 15:04:05".
	hbTime, _ := time.Parse(time.DateTime, time.Now().Truncate(time.Second).Format(time.DateTime))
	if err := d.UpdateLicenseHeartbeat(l.LicenseID, hbTime); err != nil {
		t.Fatalf("UpdateLicenseHeartbeat: %v", err)
	}

	// Verify the update was written (query directly to confirm)
	var stored string
	err := d.Conn().QueryRow("SELECT last_heartbeat_at FROM licenses WHERE license_id = ?", l.LicenseID).Scan(&stored)
	if err != nil {
		t.Fatalf("direct query: %v", err)
	}
	if stored == "" {
		t.Error("expected non-empty last_heartbeat_at after update")
	}
}


// ============================================================
// Release CRUD Tests
// ============================================================

func sampleRelease() *models.Release {
	return &models.Release{
		Component:   "kiro-client-waf",
		Channel:     "stable",
		Version:     "1.0.0",
		ArtifactURL: "https://releases.example.com/kiro-client-waf-1.0.0.tar.gz",
		SHA256:      "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		Notes:       "initial release",
		MinVersion:  "0.0.0",
		CreatedAt:   time.Now().Truncate(time.Second),
	}
}

func TestCreateRelease(t *testing.T) {
	d := newInMemoryDB(t)
	r := sampleRelease()

	if err := d.CreateRelease(r); err != nil {
		t.Fatalf("CreateRelease: %v", err)
	}
	if r.ID == 0 {
		t.Fatal("expected non-zero ID after create")
	}
}

func TestCreateRelease_DuplicateVersion(t *testing.T) {
	d := newInMemoryDB(t)
	r1 := sampleRelease()
	if err := d.CreateRelease(r1); err != nil {
		t.Fatalf("CreateRelease first: %v", err)
	}

	r2 := sampleRelease() // same component+channel+version
	err := d.CreateRelease(r2)
	if err == nil {
		t.Fatal("expected error for duplicate component+channel+version")
	}
}

func TestGetReleaseByID(t *testing.T) {
	d := newInMemoryDB(t)
	r := sampleRelease()
	if err := d.CreateRelease(r); err != nil {
		t.Fatalf("CreateRelease: %v", err)
	}

	got, err := d.GetReleaseByID(r.ID)
	if err != nil {
		t.Fatalf("GetReleaseByID: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil release")
	}
	if got.Component != r.Component {
		t.Errorf("Component = %q, want %q", got.Component, r.Component)
	}
	if got.Version != r.Version {
		t.Errorf("Version = %q, want %q", got.Version, r.Version)
	}
	if got.SHA256 != r.SHA256 {
		t.Errorf("SHA256 = %q, want %q", got.SHA256, r.SHA256)
	}
}

func TestGetReleaseByID_NotFound(t *testing.T) {
	d := newInMemoryDB(t)
	got, err := d.GetReleaseByID(9999)
	if err != nil {
		t.Fatalf("GetReleaseByID: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for non-existent ID")
	}
}

func TestGetLatestRelease(t *testing.T) {
	d := newInMemoryDB(t)

	// No releases yet
	got, err := d.GetLatestRelease("kiro-client-waf", "stable")
	if err != nil {
		t.Fatalf("GetLatestRelease empty: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil when no releases exist")
	}

	// Create two releases with different timestamps
	r1 := sampleRelease()
	r1.Version = "1.0.0"
	r1.CreatedAt = time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := d.CreateRelease(r1); err != nil {
		t.Fatalf("CreateRelease r1: %v", err)
	}

	r2 := sampleRelease()
	r2.Version = "1.1.0"
	r2.CreatedAt = time.Now().Truncate(time.Second)
	if err := d.CreateRelease(r2); err != nil {
		t.Fatalf("CreateRelease r2: %v", err)
	}

	got, err = d.GetLatestRelease("kiro-client-waf", "stable")
	if err != nil {
		t.Fatalf("GetLatestRelease: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil release")
	}
	if got.Version != "1.1.0" {
		t.Errorf("Version = %q, want %q", got.Version, "1.1.0")
	}
}

func TestGetLatestRelease_DifferentChannels(t *testing.T) {
	d := newInMemoryDB(t)

	r1 := sampleRelease()
	r1.Channel = "stable"
	r1.Version = "1.0.0"
	if err := d.CreateRelease(r1); err != nil {
		t.Fatalf("CreateRelease stable: %v", err)
	}

	r2 := sampleRelease()
	r2.Channel = "beta"
	r2.Version = "2.0.0-beta"
	if err := d.CreateRelease(r2); err != nil {
		t.Fatalf("CreateRelease beta: %v", err)
	}

	got, err := d.GetLatestRelease("kiro-client-waf", "stable")
	if err != nil {
		t.Fatalf("GetLatestRelease stable: %v", err)
	}
	if got.Version != "1.0.0" {
		t.Errorf("stable Version = %q, want %q", got.Version, "1.0.0")
	}

	got, err = d.GetLatestRelease("kiro-client-waf", "beta")
	if err != nil {
		t.Fatalf("GetLatestRelease beta: %v", err)
	}
	if got.Version != "2.0.0-beta" {
		t.Errorf("beta Version = %q, want %q", got.Version, "2.0.0-beta")
	}
}

func TestListReleases(t *testing.T) {
	d := newInMemoryDB(t)

	// Empty list
	list, err := d.ListReleases()
	if err != nil {
		t.Fatalf("ListReleases empty: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 releases, got %d", len(list))
	}

	r1 := sampleRelease()
	r1.Version = "1.0.0"
	r1.CreatedAt = time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := d.CreateRelease(r1); err != nil {
		t.Fatalf("CreateRelease r1: %v", err)
	}

	r2 := sampleRelease()
	r2.Version = "1.1.0"
	r2.CreatedAt = time.Now().Truncate(time.Second)
	if err := d.CreateRelease(r2); err != nil {
		t.Fatalf("CreateRelease r2: %v", err)
	}

	list, err = d.ListReleases()
	if err != nil {
		t.Fatalf("ListReleases: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(list))
	}
	// Ordered by created_at DESC
	if list[0].Version != "1.1.0" {
		t.Errorf("first release version = %q, want %q", list[0].Version, "1.1.0")
	}
}

func TestDeleteRelease(t *testing.T) {
	d := newInMemoryDB(t)
	r := sampleRelease()
	if err := d.CreateRelease(r); err != nil {
		t.Fatalf("CreateRelease: %v", err)
	}

	if err := d.DeleteRelease(r.ID); err != nil {
		t.Fatalf("DeleteRelease: %v", err)
	}

	got, err := d.GetReleaseByID(r.ID)
	if err != nil {
		t.Fatalf("GetReleaseByID after delete: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestDeleteRelease_NotFound(t *testing.T) {
	d := newInMemoryDB(t)
	err := d.DeleteRelease(9999)
	if err == nil {
		t.Fatal("expected error for non-existent release")
	}
}


// ============================================================
// Heartbeat Tests
// ============================================================

func sampleHeartbeat(licenseID string) *models.Heartbeat {
	return &models.Heartbeat{
		LicenseID:       licenseID,
		NodeID:          "node-001",
		ClientIP:        "10.0.0.1",
		FingerprintHash: "fp-hash-hb",
		Stats:           map[string]any{"requests": 1234, "blocked": 56},
		CreatedAt:       time.Now().Truncate(time.Second),
	}
}

func TestLogHeartbeat(t *testing.T) {
	d := newInMemoryDB(t)
	h := sampleHeartbeat("lic-001")

	if err := d.LogHeartbeat(h); err != nil {
		t.Fatalf("LogHeartbeat: %v", err)
	}
	if h.ID == 0 {
		t.Fatal("expected non-zero ID after log")
	}
}

func TestLogHeartbeat_NilStats(t *testing.T) {
	d := newInMemoryDB(t)
	h := sampleHeartbeat("lic-001")
	h.Stats = nil

	if err := d.LogHeartbeat(h); err != nil {
		t.Fatalf("LogHeartbeat with nil stats: %v", err)
	}
	if h.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
}

func TestListHeartbeats(t *testing.T) {
	d := newInMemoryDB(t)

	// Empty
	list, err := d.ListHeartbeats(10)
	if err != nil {
		t.Fatalf("ListHeartbeats empty: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 heartbeats, got %d", len(list))
	}

	// Add heartbeats
	for i := 0; i < 5; i++ {
		h := sampleHeartbeat(fmt.Sprintf("lic-%03d", i))
		h.CreatedAt = time.Now().Add(time.Duration(i) * time.Second).Truncate(time.Second)
		if err := d.LogHeartbeat(h); err != nil {
			t.Fatalf("LogHeartbeat %d: %v", i, err)
		}
	}

	// List with limit
	list, err = d.ListHeartbeats(3)
	if err != nil {
		t.Fatalf("ListHeartbeats: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 heartbeats, got %d", len(list))
	}
	// Should be ordered by created_at DESC
	if list[0].LicenseID != "lic-004" {
		t.Errorf("first heartbeat license = %q, want lic-004", list[0].LicenseID)
	}
}

func TestListHeartbeats_StatsDeserialization(t *testing.T) {
	d := newInMemoryDB(t)
	h := sampleHeartbeat("lic-001")
	h.Stats = map[string]any{"cpu": 45.5, "memory": "2GB"}
	if err := d.LogHeartbeat(h); err != nil {
		t.Fatalf("LogHeartbeat: %v", err)
	}

	list, err := d.ListHeartbeats(10)
	if err != nil {
		t.Fatalf("ListHeartbeats: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 heartbeat, got %d", len(list))
	}
	if list[0].Stats["memory"] != "2GB" {
		t.Errorf("Stats[memory] = %v, want 2GB", list[0].Stats["memory"])
	}
}

func TestListHeartbeatsByLicense(t *testing.T) {
	d := newInMemoryDB(t)

	// Create heartbeats for different licenses
	for i := 0; i < 3; i++ {
		h := sampleHeartbeat("lic-AAA")
		h.NodeID = fmt.Sprintf("node-%d", i)
		h.CreatedAt = time.Now().Add(time.Duration(i) * time.Second).Truncate(time.Second)
		if err := d.LogHeartbeat(h); err != nil {
			t.Fatalf("LogHeartbeat AAA %d: %v", i, err)
		}
	}
	for i := 0; i < 2; i++ {
		h := sampleHeartbeat("lic-BBB")
		h.NodeID = fmt.Sprintf("node-b-%d", i)
		h.CreatedAt = time.Now().Add(time.Duration(i) * time.Second).Truncate(time.Second)
		if err := d.LogHeartbeat(h); err != nil {
			t.Fatalf("LogHeartbeat BBB %d: %v", i, err)
		}
	}

	// Query for lic-AAA
	list, err := d.ListHeartbeatsByLicense("lic-AAA", 10)
	if err != nil {
		t.Fatalf("ListHeartbeatsByLicense: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 heartbeats for lic-AAA, got %d", len(list))
	}
	for _, h := range list {
		if h.LicenseID != "lic-AAA" {
			t.Errorf("unexpected license_id = %q", h.LicenseID)
		}
	}

	// Query for lic-BBB
	list, err = d.ListHeartbeatsByLicense("lic-BBB", 10)
	if err != nil {
		t.Fatalf("ListHeartbeatsByLicense BBB: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 heartbeats for lic-BBB, got %d", len(list))
	}

	// Query for non-existent license
	list, err = d.ListHeartbeatsByLicense("lic-NONE", 10)
	if err != nil {
		t.Fatalf("ListHeartbeatsByLicense NONE: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 heartbeats for lic-NONE, got %d", len(list))
	}
}

func TestCountRecentHeartbeats(t *testing.T) {
	d := newInMemoryDB(t)

	// No heartbeats
	count, err := d.CountRecentHeartbeats(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("CountRecentHeartbeats: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}

	// Add heartbeats at different times
	now := time.Now().Truncate(time.Second)
	for i := 0; i < 5; i++ {
		h := sampleHeartbeat(fmt.Sprintf("lic-%d", i))
		h.CreatedAt = now.Add(-time.Duration(i) * 10 * time.Minute)
		if err := d.LogHeartbeat(h); err != nil {
			t.Fatalf("LogHeartbeat %d: %v", i, err)
		}
	}

	// Count heartbeats in last 25 minutes (should get 3: at 0, -10, -20 min)
	count, err = d.CountRecentHeartbeats(now.Add(-25 * time.Minute))
	if err != nil {
		t.Fatalf("CountRecentHeartbeats: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 recent heartbeats, got %d", count)
	}

	// Count all heartbeats in last 2 hours
	count, err = d.CountRecentHeartbeats(now.Add(-2 * time.Hour))
	if err != nil {
		t.Fatalf("CountRecentHeartbeats all: %v", err)
	}
	if count != 5 {
		t.Fatalf("expected 5 heartbeats, got %d", count)
	}
}


// ============================================================
// Concurrent Writes (WAL Mode) Tests
// ============================================================

func TestConcurrentLicenseWrites(t *testing.T) {
	// Use temp file DB to properly test WAL mode (not :memory:)
	d := newTestDB(t)

	const numGoroutines = 20
	var wg sync.WaitGroup
	errs := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			l := &models.License{
				LicenseID:       fmt.Sprintf("lic-concurrent-%d", idx),
				LicenseKey:      fmt.Sprintf("key-concurrent-%d", idx),
				CustomerID:      fmt.Sprintf("cust-%d", idx),
				CustomerName:    fmt.Sprintf("Customer %d", idx),
				ClientIP:        fmt.Sprintf("10.0.0.%d", idx),
				FingerprintHash: fmt.Sprintf("fp-%d", idx),
				Plan:            "community",
				Status:          "active",
				ValidDays:       365,
				CreatedAt:       time.Now().Truncate(time.Second),
				ExpiresAt:       time.Now().Add(365 * 24 * time.Hour).Truncate(time.Second),
				Notes:           fmt.Sprintf("concurrent test %d", idx),
			}
			if err := d.CreateLicense(l); err != nil {
				errs <- fmt.Errorf("goroutine %d: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent write error: %v", err)
	}

	// Verify all licenses were created
	list, err := d.ListLicenses()
	if err != nil {
		t.Fatalf("ListLicenses: %v", err)
	}
	if len(list) != numGoroutines {
		t.Errorf("expected %d licenses, got %d", numGoroutines, len(list))
	}
}

func TestConcurrentHeartbeatWrites(t *testing.T) {
	d := newTestDB(t)

	const numGoroutines = 50
	var wg sync.WaitGroup
	errs := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			h := &models.Heartbeat{
				LicenseID:       fmt.Sprintf("lic-%d", idx%5), // 5 different licenses
				NodeID:          fmt.Sprintf("node-%d", idx),
				ClientIP:        fmt.Sprintf("10.0.%d.%d", idx/256, idx%256),
				FingerprintHash: fmt.Sprintf("fp-%d", idx),
				Stats:           map[string]any{"idx": idx},
				CreatedAt:       time.Now().Truncate(time.Second),
			}
			if err := d.LogHeartbeat(h); err != nil {
				errs <- fmt.Errorf("goroutine %d: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent heartbeat error: %v", err)
	}

	// Verify all heartbeats were logged
	count, err := d.CountRecentHeartbeats(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("CountRecentHeartbeats: %v", err)
	}
	if count != numGoroutines {
		t.Errorf("expected %d heartbeats, got %d", numGoroutines, count)
	}
}

func TestConcurrentMixedOperations(t *testing.T) {
	d := newTestDB(t)

	// Pre-create some licenses
	for i := 0; i < 5; i++ {
		l := &models.License{
			LicenseID:    fmt.Sprintf("lic-mix-%d", i),
			LicenseKey:   fmt.Sprintf("key-mix-%d", i),
			CustomerID:   fmt.Sprintf("cust-mix-%d", i),
			CustomerName: fmt.Sprintf("Mix Customer %d", i),
			Plan:         "community",
			Status:       "active",
			ValidDays:    365,
			CreatedAt:    time.Now().Truncate(time.Second),
			ExpiresAt:    time.Now().Add(365 * 24 * time.Hour).Truncate(time.Second),
		}
		if err := d.CreateLicense(l); err != nil {
			t.Fatalf("pre-create license %d: %v", i, err)
		}
	}

	const numOps = 30
	var wg sync.WaitGroup
	errs := make(chan error, numOps*3)

	// Concurrent heartbeat writes
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			h := &models.Heartbeat{
				LicenseID: fmt.Sprintf("lic-mix-%d", idx%5),
				NodeID:    fmt.Sprintf("node-mix-%d", idx),
				ClientIP:  "10.0.0.1",
				Stats:     map[string]any{"op": "heartbeat"},
				CreatedAt: time.Now().Truncate(time.Second),
			}
			if err := d.LogHeartbeat(h); err != nil {
				errs <- fmt.Errorf("heartbeat %d: %w", idx, err)
			}
		}(i)
	}

	// Concurrent release writes
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			r := &models.Release{
				Component:   "kiro-client-waf",
				Channel:     "stable",
				Version:     fmt.Sprintf("1.0.%d", idx),
				ArtifactURL: fmt.Sprintf("https://example.com/v1.0.%d.tar.gz", idx),
				SHA256:      fmt.Sprintf("%064d", idx),
				Notes:       fmt.Sprintf("release %d", idx),
				MinVersion:  "0.0.0",
				CreatedAt:   time.Now().Truncate(time.Second),
			}
			if err := d.CreateRelease(r); err != nil {
				errs <- fmt.Errorf("release %d: %w", idx, err)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := d.ListLicenses()
			if err != nil {
				errs <- fmt.Errorf("list licenses %d: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent mixed op error: %v", err)
	}
}

// ============================================================
// Database Initialization Tests
// ============================================================

func TestNewDB_InvalidPath(t *testing.T) {
	_, err := New("/nonexistent/path/to/db.sqlite")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestNewDB_WALMode(t *testing.T) {
	d := newTestDB(t)
	var mode string
	err := d.Conn().QueryRow("PRAGMA journal_mode").Scan(&mode)
	if err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want %q", mode, "wal")
	}
}

func TestNewDB_BusyTimeout(t *testing.T) {
	d := newTestDB(t)
	var timeout int
	err := d.Conn().QueryRow("PRAGMA busy_timeout").Scan(&timeout)
	if err != nil {
		t.Fatalf("PRAGMA busy_timeout: %v", err)
	}
	if timeout != 5000 {
		t.Errorf("busy_timeout = %d, want %d", timeout, 5000)
	}
}

func TestNewDB_ForeignKeys(t *testing.T) {
	d := newInMemoryDB(t)
	var fk int
	err := d.Conn().QueryRow("PRAGMA foreign_keys").Scan(&fk)
	if err != nil {
		t.Fatalf("PRAGMA foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}
}
