package plan

import (
	"context"
	"fmt"
	"testing"
	"time"

	"kiro_waf/internal/master/models"
)

// mockStore triển khai LicenseStore interface cho testing.
type mockStore struct {
	licenses map[string]*models.License
	nextID   int64
}

func newMockStore() *mockStore {
	return &mockStore{
		licenses: make(map[string]*models.License),
		nextID:   1,
	}
}

func (s *mockStore) CreateLicense(l *models.License) error {
	l.ID = s.nextID
	s.nextID++
	s.licenses[l.LicenseID] = l
	return nil
}

func (s *mockStore) GetLicenseByLicenseID(licenseID string) (*models.License, error) {
	l, ok := s.licenses[licenseID]
	if !ok {
		return nil, nil
	}
	return l, nil
}

func (s *mockStore) UpdateLicense(l *models.License) error {
	if _, ok := s.licenses[l.LicenseID]; !ok {
		return fmt.Errorf("not found")
	}
	s.licenses[l.LicenseID] = l
	return nil
}

func (s *mockStore) ListLicenses() ([]models.License, error) {
	var result []models.License
	for _, l := range s.licenses {
		result = append(result, *l)
	}
	return result, nil
}

func TestCreateCommunityLicense(t *testing.T) {
	store := newMockStore()
	mgr := NewManager(store)

	l, err := mgr.CreateCommunityLicense("cust-1", "Test User", "abc123fingerprint")
	if err != nil {
		t.Fatalf("CreateCommunityLicense: %v", err)
	}

	if l.Plan != PlanCommunity {
		t.Errorf("expected plan %q, got %q", PlanCommunity, l.Plan)
	}
	if l.Status != StatusActive {
		t.Errorf("expected status %q, got %q", StatusActive, l.Status)
	}
	if !l.ExpiresAt.IsZero() {
		t.Errorf("expected zero ExpiresAt for community, got %v", l.ExpiresAt)
	}
	if l.ValidDays != 0 {
		t.Errorf("expected ValidDays 0, got %d", l.ValidDays)
	}
	if l.CustomerID != "cust-1" {
		t.Errorf("expected CustomerID %q, got %q", "cust-1", l.CustomerID)
	}
	if l.FingerprintHash != "abc123fingerprint" {
		t.Errorf("expected FingerprintHash %q, got %q", "abc123fingerprint", l.FingerprintHash)
	}
	if l.LicenseID == "" {
		t.Error("expected non-empty LicenseID")
	}
	if l.LicenseKey == "" {
		t.Error("expected non-empty LicenseKey")
	}
}

func TestCheckExpiry_DowngradesExpiredPro(t *testing.T) {
	store := newMockStore()
	mgr := NewManager(store)

	// Tạo license Pro đã hết hạn
	expired := &models.License{
		LicenseID:       "lic-expired",
		LicenseKey:      "key-1",
		CustomerID:      "cust-1",
		FingerprintHash: "fp-1",
		Plan:            PlanPro,
		Status:          StatusActive,
		ValidDays:       365,
		ExpiresAt:       time.Now().Add(-1 * time.Hour), // Hết hạn 1 giờ trước
	}
	store.CreateLicense(expired)

	events, err := mgr.CheckExpiry(context.Background())
	if err != nil {
		t.Fatalf("CheckExpiry: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 downgrade event, got %d", len(events))
	}

	ev := events[0]
	if ev.LicenseID != "lic-expired" {
		t.Errorf("expected LicenseID %q, got %q", "lic-expired", ev.LicenseID)
	}
	if ev.PreviousPlan != PlanPro {
		t.Errorf("expected PreviousPlan %q, got %q", PlanPro, ev.PreviousPlan)
	}
	if ev.Reason != "expired" {
		t.Errorf("expected Reason %q, got %q", "expired", ev.Reason)
	}

	// Verify license was downgraded
	l := store.licenses["lic-expired"]
	if l.Plan != PlanCommunity {
		t.Errorf("expected plan %q after downgrade, got %q", PlanCommunity, l.Plan)
	}
	if l.Status != StatusDowngraded {
		t.Errorf("expected status %q after downgrade, got %q", StatusDowngraded, l.Status)
	}
	if !l.ExpiresAt.IsZero() {
		t.Errorf("expected zero ExpiresAt after downgrade, got %v", l.ExpiresAt)
	}
}

func TestCheckExpiry_SkipsSuspended(t *testing.T) {
	store := newMockStore()
	mgr := NewManager(store)

	// License suspended + expired → giữ suspended
	suspended := &models.License{
		LicenseID:  "lic-suspended",
		LicenseKey: "key-2",
		Plan:       PlanEnterprise,
		Status:     StatusSuspended,
		ExpiresAt:  time.Now().Add(-1 * time.Hour),
	}
	store.CreateLicense(suspended)

	events, err := mgr.CheckExpiry(context.Background())
	if err != nil {
		t.Fatalf("CheckExpiry: %v", err)
	}

	if len(events) != 0 {
		t.Fatalf("expected 0 events for suspended license, got %d", len(events))
	}

	// Verify license stays suspended
	l := store.licenses["lic-suspended"]
	if l.Status != StatusSuspended {
		t.Errorf("expected status %q, got %q", StatusSuspended, l.Status)
	}
	if l.Plan != PlanEnterprise {
		t.Errorf("expected plan %q unchanged, got %q", PlanEnterprise, l.Plan)
	}
}

func TestCheckExpiry_SkipsCommunity(t *testing.T) {
	store := newMockStore()
	mgr := NewManager(store)

	community := &models.License{
		LicenseID:  "lic-community",
		LicenseKey: "key-3",
		Plan:       PlanCommunity,
		Status:     StatusActive,
		ExpiresAt:  time.Time{}, // Zero = vô thời hạn
	}
	store.CreateLicense(community)

	events, err := mgr.CheckExpiry(context.Background())
	if err != nil {
		t.Fatalf("CheckExpiry: %v", err)
	}

	if len(events) != 0 {
		t.Fatalf("expected 0 events for community license, got %d", len(events))
	}
}

func TestUpgradePlan(t *testing.T) {
	store := newMockStore()
	mgr := NewManager(store)

	// Tạo license Community
	l, _ := mgr.CreateCommunityLicense("cust-1", "User", "fp-1")

	// Upgrade lên Pro
	err := mgr.UpgradePlan(l.LicenseID, PlanPro, 365)
	if err != nil {
		t.Fatalf("UpgradePlan: %v", err)
	}

	updated := store.licenses[l.LicenseID]
	if updated.Plan != PlanPro {
		t.Errorf("expected plan %q, got %q", PlanPro, updated.Plan)
	}
	if updated.Status != StatusActive {
		t.Errorf("expected status %q, got %q", StatusActive, updated.Status)
	}
	if updated.ValidDays != 365 {
		t.Errorf("expected ValidDays 365, got %d", updated.ValidDays)
	}
	if updated.ExpiresAt.IsZero() {
		t.Error("expected non-zero ExpiresAt after upgrade")
	}
	// Verify identity preserved
	if updated.LicenseKey != l.LicenseKey {
		t.Error("LicenseKey should be preserved after upgrade")
	}
	if updated.FingerprintHash != l.FingerprintHash {
		t.Error("FingerprintHash should be preserved after upgrade")
	}
}

func TestUpgradePlan_InvalidPlan(t *testing.T) {
	store := newMockStore()
	mgr := NewManager(store)

	l, _ := mgr.CreateCommunityLicense("cust-1", "User", "fp-1")

	err := mgr.UpgradePlan(l.LicenseID, "invalid", 365)
	if err == nil {
		t.Fatal("expected error for invalid plan")
	}
}

func TestUpgradePlan_InvalidDays(t *testing.T) {
	store := newMockStore()
	mgr := NewManager(store)

	l, _ := mgr.CreateCommunityLicense("cust-1", "User", "fp-1")

	err := mgr.UpgradePlan(l.LicenseID, PlanPro, 0)
	if err == nil {
		t.Fatal("expected error for zero validDays")
	}
}

func TestDowngradeToCommunity(t *testing.T) {
	store := newMockStore()
	mgr := NewManager(store)

	// Tạo license Pro
	l, _ := mgr.CreateCommunityLicense("cust-1", "User", "fp-1")
	mgr.UpgradePlan(l.LicenseID, PlanPro, 365)

	// Downgrade
	err := mgr.DowngradeToCommunity(l.LicenseID)
	if err != nil {
		t.Fatalf("DowngradeToCommunity: %v", err)
	}

	updated := store.licenses[l.LicenseID]
	if updated.Plan != PlanCommunity {
		t.Errorf("expected plan %q, got %q", PlanCommunity, updated.Plan)
	}
	if updated.Status != StatusDowngraded {
		t.Errorf("expected status %q, got %q", StatusDowngraded, updated.Status)
	}
	if !updated.ExpiresAt.IsZero() {
		t.Errorf("expected zero ExpiresAt, got %v", updated.ExpiresAt)
	}
	// Identity preserved
	if updated.LicenseKey != l.LicenseKey {
		t.Error("LicenseKey should be preserved after downgrade")
	}
	if updated.FingerprintHash != l.FingerprintHash {
		t.Error("FingerprintHash should be preserved after downgrade")
	}
}

func TestEnforcePlanLimits_Community(t *testing.T) {
	store := newMockStore()
	mgr := NewManager(store)

	// Within limits
	err := mgr.EnforcePlanLimits(PlanCommunity, RequestedConfig{
		Domains: 1, XDPEnabled: false, OTAEnabled: false, CustomRPM: 60,
	})
	if err != nil {
		t.Errorf("expected no error for valid community config, got: %v", err)
	}

	// Exceeds domain limit
	err = mgr.EnforcePlanLimits(PlanCommunity, RequestedConfig{Domains: 2})
	if err == nil {
		t.Error("expected error for domains exceeding community limit")
	}

	// XDP not available
	err = mgr.EnforcePlanLimits(PlanCommunity, RequestedConfig{XDPEnabled: true})
	if err == nil {
		t.Error("expected error for XDP in community plan")
	}

	// OTA not available
	err = mgr.EnforcePlanLimits(PlanCommunity, RequestedConfig{OTAEnabled: true})
	if err == nil {
		t.Error("expected error for OTA in community plan")
	}

	// RPM exceeds limit
	err = mgr.EnforcePlanLimits(PlanCommunity, RequestedConfig{CustomRPM: 120})
	if err == nil {
		t.Error("expected error for RPM exceeding community limit")
	}
}

func TestEnforcePlanLimits_Enterprise(t *testing.T) {
	store := newMockStore()
	mgr := NewManager(store)

	// Enterprise = unlimited
	err := mgr.EnforcePlanLimits(PlanEnterprise, RequestedConfig{
		Domains: 100, XDPEnabled: true, OTAEnabled: true, CustomRPM: 10000,
	})
	if err != nil {
		t.Errorf("expected no error for enterprise config, got: %v", err)
	}
}

func TestLimitsForPlan(t *testing.T) {
	tests := []struct {
		plan   string
		expect PlanLimits
	}{
		{PlanCommunity, CommunityLimits},
		{PlanPro, ProLimits},
		{PlanEnterprise, EnterpriseLimits},
		{"unknown", CommunityLimits}, // Fallback to community
	}

	for _, tt := range tests {
		got := LimitsForPlan(tt.plan)
		if got != tt.expect {
			t.Errorf("LimitsForPlan(%q) = %+v, want %+v", tt.plan, got, tt.expect)
		}
	}
}

// mockHistoryStore triển khai HistoryStore interface cho testing.
type mockHistoryStore struct {
	history map[string][]PlanChange
}

func newMockHistoryStore() *mockHistoryStore {
	return &mockHistoryStore{
		history: make(map[string][]PlanChange),
	}
}

func (h *mockHistoryStore) RecordPlanChange(licenseID string, change PlanChange) error {
	h.history[licenseID] = append(h.history[licenseID], change)
	return nil
}

func (h *mockHistoryStore) GetPlanHistory(licenseID string) ([]PlanChange, error) {
	return h.history[licenseID], nil
}

func TestCheckExpiry_RecordsPlanChangeHistory(t *testing.T) {
	store := newMockStore()
	histStore := newMockHistoryStore()
	mgr := NewManager(store, WithHistoryStore(histStore))

	// Tạo license Pro đã hết hạn
	expired := &models.License{
		LicenseID:       "lic-hist-1",
		LicenseKey:      "key-hist-1",
		CustomerID:      "cust-1",
		FingerprintHash: "fp-1",
		Plan:            PlanPro,
		Status:          StatusActive,
		ValidDays:       365,
		ExpiresAt:       time.Now().Add(-1 * time.Hour),
	}
	store.CreateLicense(expired)

	_, err := mgr.CheckExpiry(context.Background())
	if err != nil {
		t.Fatalf("CheckExpiry: %v", err)
	}

	// Verify PlanChange was recorded
	history, err := histStore.GetPlanHistory("lic-hist-1")
	if err != nil {
		t.Fatalf("GetPlanHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}

	change := history[0]
	if change.FromPlan != PlanPro {
		t.Errorf("expected FromPlan %q, got %q", PlanPro, change.FromPlan)
	}
	if change.ToPlan != PlanCommunity {
		t.Errorf("expected ToPlan %q, got %q", PlanCommunity, change.ToPlan)
	}
	if change.Reason != "expired" {
		t.Errorf("expected Reason %q, got %q", "expired", change.Reason)
	}
	if change.ChangedAt.IsZero() {
		t.Error("expected non-zero ChangedAt")
	}
}

func TestUpgradePlan_RecordsPlanChangeHistory(t *testing.T) {
	store := newMockStore()
	histStore := newMockHistoryStore()
	mgr := NewManager(store, WithHistoryStore(histStore))

	// Tạo license Community
	l, _ := mgr.CreateCommunityLicense("cust-1", "User", "fp-1")

	// Upgrade lên Pro
	err := mgr.UpgradePlan(l.LicenseID, PlanPro, 365)
	if err != nil {
		t.Fatalf("UpgradePlan: %v", err)
	}

	// Verify PlanChange was recorded
	history, err := histStore.GetPlanHistory(l.LicenseID)
	if err != nil {
		t.Fatalf("GetPlanHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}

	change := history[0]
	if change.FromPlan != PlanCommunity {
		t.Errorf("expected FromPlan %q, got %q", PlanCommunity, change.FromPlan)
	}
	if change.ToPlan != PlanPro {
		t.Errorf("expected ToPlan %q, got %q", PlanPro, change.ToPlan)
	}
	if change.Reason != "admin_upgrade" {
		t.Errorf("expected Reason %q, got %q", "admin_upgrade", change.Reason)
	}
}

func TestDowngradeToCommunity_RecordsPlanChangeHistory(t *testing.T) {
	store := newMockStore()
	histStore := newMockHistoryStore()
	mgr := NewManager(store, WithHistoryStore(histStore))

	// Tạo license Pro
	l, _ := mgr.CreateCommunityLicense("cust-1", "User", "fp-1")
	mgr.UpgradePlan(l.LicenseID, PlanPro, 365)

	// Clear history from upgrade
	histStore.history[l.LicenseID] = nil

	// Downgrade
	err := mgr.DowngradeToCommunity(l.LicenseID)
	if err != nil {
		t.Fatalf("DowngradeToCommunity: %v", err)
	}

	// Verify PlanChange was recorded
	history, err := histStore.GetPlanHistory(l.LicenseID)
	if err != nil {
		t.Fatalf("GetPlanHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}

	change := history[0]
	if change.FromPlan != PlanPro {
		t.Errorf("expected FromPlan %q, got %q", PlanPro, change.FromPlan)
	}
	if change.ToPlan != PlanCommunity {
		t.Errorf("expected ToPlan %q, got %q", PlanCommunity, change.ToPlan)
	}
	if change.Reason != "admin_downgrade" {
		t.Errorf("expected Reason %q, got %q", "admin_downgrade", change.Reason)
	}
}

func TestNoHistoryStore_DoesNotPanic(t *testing.T) {
	store := newMockStore()
	// No history store — should not panic
	mgr := NewManager(store)

	l, _ := mgr.CreateCommunityLicense("cust-1", "User", "fp-1")
	err := mgr.UpgradePlan(l.LicenseID, PlanPro, 365)
	if err != nil {
		t.Fatalf("UpgradePlan without history store: %v", err)
	}

	err = mgr.DowngradeToCommunity(l.LicenseID)
	if err != nil {
		t.Fatalf("DowngradeToCommunity without history store: %v", err)
	}
}

func TestMultiplePlanChanges_FullHistory(t *testing.T) {
	store := newMockStore()
	histStore := newMockHistoryStore()
	mgr := NewManager(store, WithHistoryStore(histStore))

	// Create → Upgrade to Pro → Downgrade → Upgrade to Enterprise
	l, _ := mgr.CreateCommunityLicense("cust-1", "User", "fp-1")

	mgr.UpgradePlan(l.LicenseID, PlanPro, 365)
	mgr.DowngradeToCommunity(l.LicenseID)
	mgr.UpgradePlan(l.LicenseID, PlanEnterprise, 365)

	history, _ := histStore.GetPlanHistory(l.LicenseID)
	if len(history) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(history))
	}

	// Verify sequence
	if history[0].FromPlan != PlanCommunity || history[0].ToPlan != PlanPro {
		t.Errorf("entry 0: expected community→pro, got %s→%s", history[0].FromPlan, history[0].ToPlan)
	}
	if history[1].FromPlan != PlanPro || history[1].ToPlan != PlanCommunity {
		t.Errorf("entry 1: expected pro→community, got %s→%s", history[1].FromPlan, history[1].ToPlan)
	}
	if history[2].FromPlan != PlanCommunity || history[2].ToPlan != PlanEnterprise {
		t.Errorf("entry 2: expected community→enterprise, got %s→%s", history[2].FromPlan, history[2].ToPlan)
	}
}

func TestStartPeriodicExpiryChecker(t *testing.T) {
	store := newMockStore()
	histStore := newMockHistoryStore()
	mgr := NewManager(store, WithHistoryStore(histStore))

	// Tạo license Pro đã hết hạn
	expired := &models.License{
		LicenseID:       "lic-periodic-1",
		LicenseKey:      "key-p1",
		CustomerID:      "cust-1",
		FingerprintHash: "fp-1",
		Plan:            PlanPro,
		Status:          StatusActive,
		ValidDays:       365,
		ExpiresAt:       time.Now().Add(-1 * time.Hour),
	}
	store.CreateLicense(expired)

	// Start periodic checker with a short-lived context
	ctx, cancel := context.WithCancel(context.Background())
	ch := mgr.StartPeriodicExpiryChecker(ctx)

	// Cancel immediately — the goroutine should stop
	cancel()

	// Drain channel to verify it closes
	for range ch {
		// consume any events
	}

	// Verify the goroutine stopped (channel closed)
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed after context cancel")
	}
}

func TestStartPeriodicExpiryChecker_ContextCancel(t *testing.T) {
	store := newMockStore()
	mgr := NewManager(store)

	ctx, cancel := context.WithCancel(context.Background())
	ch := mgr.StartPeriodicExpiryChecker(ctx)

	// Cancel context
	cancel()

	// Channel should eventually close
	select {
	case _, ok := <-ch:
		if ok {
			// Got events, that's fine, keep draining
			for range ch {
			}
		}
		// Channel closed, test passes
	case <-time.After(2 * time.Second):
		t.Fatal("periodic checker did not stop within 2 seconds after context cancel")
	}
}
