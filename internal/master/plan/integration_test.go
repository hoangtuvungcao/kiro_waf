package plan_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"kiro_waf/internal/master/db"
	"kiro_waf/internal/master/models"
	"kiro_waf/internal/master/plan"
)

// newIntegrationDB creates a real SQLite database for integration testing.
func newIntegrationDB(t *testing.T) *db.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "integration_test.db")
	d, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("newIntegrationDB: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// dbLicenseStore adapts db.DB to plan.LicenseStore interface.
type dbLicenseStore struct {
	db *db.DB
}

func (s *dbLicenseStore) CreateLicense(l *models.License) error {
	return s.db.CreateLicense(l)
}

func (s *dbLicenseStore) GetLicenseByLicenseID(licenseID string) (*models.License, error) {
	return s.db.GetLicenseByLicenseID(licenseID)
}

func (s *dbLicenseStore) UpdateLicense(l *models.License) error {
	return s.db.UpdateLicense(l)
}

func (s *dbLicenseStore) ListLicenses() ([]models.License, error) {
	return s.db.ListLicenses()
}


// =============================================================================
// Integration Test: License Expiry → Auto-Downgrade to Community within 60s
// Requirements: 14.2
// =============================================================================

func TestIntegration_ExpiryAutoDowngrade(t *testing.T) {
	database := newIntegrationDB(t)
	store := &dbLicenseStore{db: database}
	mgr := plan.NewManager(store)

	// Create a Pro license that expired 1 minute ago
	now := time.Now().UTC()
	proLicense := &models.License{
		LicenseID:       "lic-int-pro-001",
		LicenseKey:      "key-int-pro-001",
		CustomerID:      "cust-int-001",
		CustomerName:    "Integration Test User",
		FingerprintHash: "fp-integration-abc123",
		Plan:            "pro",
		Status:          "active",
		ValidDays:       365,
		CreatedAt:       now.Add(-400 * 24 * time.Hour),
		ExpiresAt:       now.Add(-1 * time.Minute), // Expired 1 minute ago
	}
	if err := database.CreateLicense(proLicense); err != nil {
		t.Fatalf("seed pro license: %v", err)
	}

	// Run CheckExpiry — should downgrade within the call (simulating the 60s periodic check)
	events, err := mgr.CheckExpiry(context.Background())
	if err != nil {
		t.Fatalf("CheckExpiry: %v", err)
	}

	// Verify downgrade event was produced
	if len(events) != 1 {
		t.Fatalf("expected 1 downgrade event, got %d", len(events))
	}
	ev := events[0]
	if ev.LicenseID != "lic-int-pro-001" {
		t.Errorf("event LicenseID = %q, want %q", ev.LicenseID, "lic-int-pro-001")
	}
	if ev.PreviousPlan != "pro" {
		t.Errorf("event PreviousPlan = %q, want %q", ev.PreviousPlan, "pro")
	}
	if ev.Reason != "expired" {
		t.Errorf("event Reason = %q, want %q", ev.Reason, "expired")
	}

	// Verify the license in DB was actually downgraded
	updated, err := database.GetLicenseByLicenseID("lic-int-pro-001")
	if err != nil {
		t.Fatalf("GetLicenseByLicenseID: %v", err)
	}
	if updated.Plan != "community" {
		t.Errorf("license Plan = %q, want %q", updated.Plan, "community")
	}
	if updated.Status != "downgraded" {
		t.Errorf("license Status = %q, want %q", updated.Status, "downgraded")
	}
	if !updated.ExpiresAt.IsZero() {
		t.Errorf("license ExpiresAt should be zero (perpetual), got %v", updated.ExpiresAt)
	}
	// Identity fields preserved
	if updated.LicenseKey != "key-int-pro-001" {
		t.Errorf("LicenseKey changed: got %q, want %q", updated.LicenseKey, "key-int-pro-001")
	}
	if updated.FingerprintHash != "fp-integration-abc123" {
		t.Errorf("FingerprintHash changed: got %q, want %q", updated.FingerprintHash, "fp-integration-abc123")
	}
}

// TestIntegration_ExpiryAutoDowngrade_Enterprise tests Enterprise license expiry.
func TestIntegration_ExpiryAutoDowngrade_Enterprise(t *testing.T) {
	database := newIntegrationDB(t)
	store := &dbLicenseStore{db: database}
	mgr := plan.NewManager(store)

	now := time.Now().UTC()
	entLicense := &models.License{
		LicenseID:       "lic-int-ent-001",
		LicenseKey:      "key-int-ent-001",
		CustomerID:      "cust-int-002",
		CustomerName:    "Enterprise User",
		FingerprintHash: "fp-enterprise-xyz789",
		Plan:            "enterprise",
		Status:          "active",
		ValidDays:       3650,
		CreatedAt:       now.Add(-4000 * 24 * time.Hour),
		ExpiresAt:       now.Add(-5 * time.Second), // Expired 5 seconds ago
	}
	if err := database.CreateLicense(entLicense); err != nil {
		t.Fatalf("seed enterprise license: %v", err)
	}

	events, err := mgr.CheckExpiry(context.Background())
	if err != nil {
		t.Fatalf("CheckExpiry: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 downgrade event, got %d", len(events))
	}

	// Verify DB state
	updated, err := database.GetLicenseByLicenseID("lic-int-ent-001")
	if err != nil {
		t.Fatalf("GetLicenseByLicenseID: %v", err)
	}
	if updated.Plan != "community" {
		t.Errorf("license Plan = %q, want %q", updated.Plan, "community")
	}
	if updated.Status != "downgraded" {
		t.Errorf("license Status = %q, want %q", updated.Status, "downgraded")
	}
	if updated.LicenseKey != "key-int-ent-001" {
		t.Errorf("LicenseKey changed after downgrade")
	}
	if updated.FingerprintHash != "fp-enterprise-xyz789" {
		t.Errorf("FingerprintHash changed after downgrade")
	}
}


// =============================================================================
// Integration Test: Upgrade/Downgrade Preserves Identity Fields
// Requirements: 14.3, 14.4
// =============================================================================

func TestIntegration_UpgradeDowngrade_PreservesIdentity(t *testing.T) {
	database := newIntegrationDB(t)
	store := &dbLicenseStore{db: database}
	mgr := plan.NewManager(store)

	// Create a Community license via PlanManager
	original, err := mgr.CreateCommunityLicense("cust-identity-001", "Identity Test User", "fp-identity-unique-hash")
	if err != nil {
		t.Fatalf("CreateCommunityLicense: %v", err)
	}

	originalKey := original.LicenseKey
	originalFP := original.FingerprintHash
	originalLicenseID := original.LicenseID

	// Verify initial state
	if original.Plan != "community" {
		t.Fatalf("initial plan = %q, want community", original.Plan)
	}
	if original.Status != "active" {
		t.Fatalf("initial status = %q, want active", original.Status)
	}

	// Upgrade to Pro
	err = mgr.UpgradePlan(originalLicenseID, "pro", 365)
	if err != nil {
		t.Fatalf("UpgradePlan to pro: %v", err)
	}

	// Verify identity preserved after upgrade
	afterUpgrade, err := database.GetLicenseByLicenseID(originalLicenseID)
	if err != nil {
		t.Fatalf("GetLicenseByLicenseID after upgrade: %v", err)
	}
	if afterUpgrade.LicenseKey != originalKey {
		t.Errorf("LicenseKey changed after upgrade: got %q, want %q", afterUpgrade.LicenseKey, originalKey)
	}
	if afterUpgrade.FingerprintHash != originalFP {
		t.Errorf("FingerprintHash changed after upgrade: got %q, want %q", afterUpgrade.FingerprintHash, originalFP)
	}
	if afterUpgrade.Plan != "pro" {
		t.Errorf("Plan after upgrade = %q, want pro", afterUpgrade.Plan)
	}
	if afterUpgrade.Status != "active" {
		t.Errorf("Status after upgrade = %q, want active", afterUpgrade.Status)
	}
	if afterUpgrade.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should be non-zero after upgrade to Pro")
	}

	// Downgrade back to Community
	err = mgr.DowngradeToCommunity(originalLicenseID)
	if err != nil {
		t.Fatalf("DowngradeToCommunity: %v", err)
	}

	// Verify identity preserved after downgrade
	afterDowngrade, err := database.GetLicenseByLicenseID(originalLicenseID)
	if err != nil {
		t.Fatalf("GetLicenseByLicenseID after downgrade: %v", err)
	}
	if afterDowngrade.LicenseKey != originalKey {
		t.Errorf("LicenseKey changed after downgrade: got %q, want %q", afterDowngrade.LicenseKey, originalKey)
	}
	if afterDowngrade.FingerprintHash != originalFP {
		t.Errorf("FingerprintHash changed after downgrade: got %q, want %q", afterDowngrade.FingerprintHash, originalFP)
	}
	if afterDowngrade.Plan != "community" {
		t.Errorf("Plan after downgrade = %q, want community", afterDowngrade.Plan)
	}
	if afterDowngrade.Status != "downgraded" {
		t.Errorf("Status after downgrade = %q, want downgraded", afterDowngrade.Status)
	}
	if !afterDowngrade.ExpiresAt.IsZero() {
		t.Errorf("ExpiresAt should be zero after downgrade, got %v", afterDowngrade.ExpiresAt)
	}

	// Upgrade again to Enterprise — identity still preserved
	err = mgr.UpgradePlan(originalLicenseID, "enterprise", 3650)
	if err != nil {
		t.Fatalf("UpgradePlan to enterprise: %v", err)
	}

	afterReUpgrade, err := database.GetLicenseByLicenseID(originalLicenseID)
	if err != nil {
		t.Fatalf("GetLicenseByLicenseID after re-upgrade: %v", err)
	}
	if afterReUpgrade.LicenseKey != originalKey {
		t.Errorf("LicenseKey changed after re-upgrade: got %q, want %q", afterReUpgrade.LicenseKey, originalKey)
	}
	if afterReUpgrade.FingerprintHash != originalFP {
		t.Errorf("FingerprintHash changed after re-upgrade: got %q, want %q", afterReUpgrade.FingerprintHash, originalFP)
	}
	if afterReUpgrade.Plan != "enterprise" {
		t.Errorf("Plan after re-upgrade = %q, want enterprise", afterReUpgrade.Plan)
	}
}


// =============================================================================
// Integration Test: Suspended + Expired → Keeps Suspended
// Requirements: 14.7, 14.9
// =============================================================================

func TestIntegration_SuspendedPlusExpired_KeepsSuspended(t *testing.T) {
	database := newIntegrationDB(t)
	store := &dbLicenseStore{db: database}
	mgr := plan.NewManager(store)

	now := time.Now().UTC()

	// Create a Pro license that is both suspended AND expired
	suspendedExpired := &models.License{
		LicenseID:       "lic-int-susp-exp-001",
		LicenseKey:      "key-susp-exp-001",
		CustomerID:      "cust-int-003",
		CustomerName:    "Suspended Expired User",
		FingerprintHash: "fp-susp-exp-hash",
		Plan:            "pro",
		Status:          "suspended",
		ValidDays:       365,
		CreatedAt:       now.Add(-400 * 24 * time.Hour),
		ExpiresAt:       now.Add(-10 * time.Minute), // Expired 10 minutes ago
	}
	if err := database.CreateLicense(suspendedExpired); err != nil {
		t.Fatalf("seed suspended+expired license: %v", err)
	}

	// Also create an active expired license to ensure CheckExpiry processes correctly
	activeExpired := &models.License{
		LicenseID:       "lic-int-active-exp-001",
		LicenseKey:      "key-active-exp-001",
		CustomerID:      "cust-int-004",
		CustomerName:    "Active Expired User",
		FingerprintHash: "fp-active-exp-hash",
		Plan:            "enterprise",
		Status:          "active",
		ValidDays:       3650,
		CreatedAt:       now.Add(-4000 * 24 * time.Hour),
		ExpiresAt:       now.Add(-2 * time.Hour), // Expired 2 hours ago
	}
	if err := database.CreateLicense(activeExpired); err != nil {
		t.Fatalf("seed active+expired license: %v", err)
	}

	// Run CheckExpiry
	events, err := mgr.CheckExpiry(context.Background())
	if err != nil {
		t.Fatalf("CheckExpiry: %v", err)
	}

	// Only the active expired license should be downgraded, NOT the suspended one
	if len(events) != 1 {
		t.Fatalf("expected 1 downgrade event (only active expired), got %d", len(events))
	}
	if events[0].LicenseID != "lic-int-active-exp-001" {
		t.Errorf("downgraded wrong license: got %q, want %q", events[0].LicenseID, "lic-int-active-exp-001")
	}

	// Verify suspended license remains unchanged
	suspLicense, err := database.GetLicenseByLicenseID("lic-int-susp-exp-001")
	if err != nil {
		t.Fatalf("GetLicenseByLicenseID suspended: %v", err)
	}
	if suspLicense.Status != "suspended" {
		t.Errorf("suspended license status changed to %q, should remain suspended", suspLicense.Status)
	}
	if suspLicense.Plan != "pro" {
		t.Errorf("suspended license plan changed to %q, should remain pro", suspLicense.Plan)
	}

	// Verify active expired license was downgraded
	activeLicense, err := database.GetLicenseByLicenseID("lic-int-active-exp-001")
	if err != nil {
		t.Fatalf("GetLicenseByLicenseID active expired: %v", err)
	}
	if activeLicense.Status != "downgraded" {
		t.Errorf("active expired license status = %q, want downgraded", activeLicense.Status)
	}
	if activeLicense.Plan != "community" {
		t.Errorf("active expired license plan = %q, want community", activeLicense.Plan)
	}
}

// TestIntegration_SuspendedEnterprise_Expired tests Enterprise suspended + expired.
func TestIntegration_SuspendedEnterprise_Expired(t *testing.T) {
	database := newIntegrationDB(t)
	store := &dbLicenseStore{db: database}
	mgr := plan.NewManager(store)

	now := time.Now().UTC()

	// Enterprise license: suspended AND expired
	l := &models.License{
		LicenseID:       "lic-int-ent-susp-001",
		LicenseKey:      "key-ent-susp-001",
		CustomerID:      "cust-int-005",
		CustomerName:    "Enterprise Suspended",
		FingerprintHash: "fp-ent-susp-hash",
		Plan:            "enterprise",
		Status:          "suspended",
		ValidDays:       3650,
		CreatedAt:       now.Add(-5000 * 24 * time.Hour),
		ExpiresAt:       now.Add(-1 * 24 * time.Hour), // Expired yesterday
	}
	if err := database.CreateLicense(l); err != nil {
		t.Fatalf("seed license: %v", err)
	}

	events, err := mgr.CheckExpiry(context.Background())
	if err != nil {
		t.Fatalf("CheckExpiry: %v", err)
	}

	// No events — suspended takes priority over expired
	if len(events) != 0 {
		t.Fatalf("expected 0 events for suspended+expired, got %d", len(events))
	}

	// Verify license unchanged
	updated, err := database.GetLicenseByLicenseID("lic-int-ent-susp-001")
	if err != nil {
		t.Fatalf("GetLicenseByLicenseID: %v", err)
	}
	if updated.Status != "suspended" {
		t.Errorf("status = %q, want suspended", updated.Status)
	}
	if updated.Plan != "enterprise" {
		t.Errorf("plan = %q, want enterprise", updated.Plan)
	}
}


// =============================================================================
// Integration Test: Plan Enforcement via EnforcePlanLimits with Real DB
// Requirements: 14.5, 14.8, 14.10
// =============================================================================

func TestIntegration_PlanEnforcement_CommunityLimits(t *testing.T) {
	database := newIntegrationDB(t)
	store := &dbLicenseStore{db: database}
	mgr := plan.NewManager(store)

	// Create a Community license
	l, err := mgr.CreateCommunityLicense("cust-enforce-001", "Enforce User", "fp-enforce-001")
	if err != nil {
		t.Fatalf("CreateCommunityLicense: %v", err)
	}

	// Verify Community license has correct plan
	fromDB, err := database.GetLicenseByLicenseID(l.LicenseID)
	if err != nil {
		t.Fatalf("GetLicenseByLicenseID: %v", err)
	}
	if fromDB.Plan != "community" {
		t.Fatalf("plan = %q, want community", fromDB.Plan)
	}

	// Test enforcement: Community allows 1 domain, 60 RPM, no XDP, no OTA
	tests := []struct {
		name      string
		config    plan.RequestedConfig
		wantError bool
	}{
		{
			name:      "within community limits",
			config:    plan.RequestedConfig{Domains: 1, XDPEnabled: false, OTAEnabled: false, CustomRPM: 60},
			wantError: false,
		},
		{
			name:      "exceeds domain limit",
			config:    plan.RequestedConfig{Domains: 2, XDPEnabled: false, OTAEnabled: false, CustomRPM: 60},
			wantError: true,
		},
		{
			name:      "XDP not allowed in community",
			config:    plan.RequestedConfig{Domains: 1, XDPEnabled: true, OTAEnabled: false, CustomRPM: 60},
			wantError: true,
		},
		{
			name:      "OTA not allowed in community",
			config:    plan.RequestedConfig{Domains: 1, XDPEnabled: false, OTAEnabled: true, CustomRPM: 60},
			wantError: true,
		},
		{
			name:      "RPM exceeds community limit",
			config:    plan.RequestedConfig{Domains: 1, XDPEnabled: false, OTAEnabled: false, CustomRPM: 120},
			wantError: true,
		},
		{
			name:      "zero RPM within limits",
			config:    plan.RequestedConfig{Domains: 1, XDPEnabled: false, OTAEnabled: false, CustomRPM: 0},
			wantError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := mgr.EnforcePlanLimits(fromDB.Plan, tc.config)
			if tc.wantError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tc.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestIntegration_PlanEnforcement_AfterUpgrade(t *testing.T) {
	database := newIntegrationDB(t)
	store := &dbLicenseStore{db: database}
	mgr := plan.NewManager(store)

	// Create Community license and upgrade to Pro
	l, err := mgr.CreateCommunityLicense("cust-enforce-002", "Pro User", "fp-enforce-002")
	if err != nil {
		t.Fatalf("CreateCommunityLicense: %v", err)
	}

	err = mgr.UpgradePlan(l.LicenseID, "pro", 365)
	if err != nil {
		t.Fatalf("UpgradePlan: %v", err)
	}

	// Pro allows: 5 domains, XDP, OTA, 120 RPM
	err = mgr.EnforcePlanLimits("pro", plan.RequestedConfig{
		Domains: 5, XDPEnabled: true, OTAEnabled: true, CustomRPM: 120,
	})
	if err != nil {
		t.Errorf("Pro config within limits should pass: %v", err)
	}

	// Pro exceeds: 6 domains
	err = mgr.EnforcePlanLimits("pro", plan.RequestedConfig{
		Domains: 6, XDPEnabled: true, OTAEnabled: true, CustomRPM: 120,
	})
	if err == nil {
		t.Error("Pro config exceeding 5 domains should fail")
	}

	// Pro exceeds: 200 RPM
	err = mgr.EnforcePlanLimits("pro", plan.RequestedConfig{
		Domains: 1, XDPEnabled: false, OTAEnabled: false, CustomRPM: 200,
	})
	if err == nil {
		t.Error("Pro config exceeding 120 RPM should fail")
	}
}

func TestIntegration_PlanEnforcement_AfterDowngrade(t *testing.T) {
	database := newIntegrationDB(t)
	store := &dbLicenseStore{db: database}
	mgr := plan.NewManager(store)

	// Create, upgrade to Enterprise, then downgrade
	l, err := mgr.CreateCommunityLicense("cust-enforce-003", "Downgrade User", "fp-enforce-003")
	if err != nil {
		t.Fatalf("CreateCommunityLicense: %v", err)
	}

	err = mgr.UpgradePlan(l.LicenseID, "enterprise", 3650)
	if err != nil {
		t.Fatalf("UpgradePlan: %v", err)
	}

	// Enterprise: unlimited
	err = mgr.EnforcePlanLimits("enterprise", plan.RequestedConfig{
		Domains: 100, XDPEnabled: true, OTAEnabled: true, CustomRPM: 10000,
	})
	if err != nil {
		t.Errorf("Enterprise unlimited config should pass: %v", err)
	}

	// Downgrade to Community
	err = mgr.DowngradeToCommunity(l.LicenseID)
	if err != nil {
		t.Fatalf("DowngradeToCommunity: %v", err)
	}

	// After downgrade, community limits apply
	fromDB, err := database.GetLicenseByLicenseID(l.LicenseID)
	if err != nil {
		t.Fatalf("GetLicenseByLicenseID: %v", err)
	}

	err = mgr.EnforcePlanLimits(fromDB.Plan, plan.RequestedConfig{
		Domains: 2, XDPEnabled: false, OTAEnabled: false, CustomRPM: 60,
	})
	if err == nil {
		t.Error("After downgrade to community, 2 domains should be rejected")
	}

	err = mgr.EnforcePlanLimits(fromDB.Plan, plan.RequestedConfig{
		Domains: 1, XDPEnabled: false, OTAEnabled: false, CustomRPM: 60,
	})
	if err != nil {
		t.Errorf("After downgrade, valid community config should pass: %v", err)
	}
}
