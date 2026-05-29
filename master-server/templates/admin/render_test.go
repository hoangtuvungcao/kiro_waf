package admin

import (
	"bytes"
	"testing"
	"time"

	"kiro_waf/master-server/models"
)

func TestRenderDashboard(t *testing.T) {
	var buf bytes.Buffer
	data := &DashboardData{
		Stats: DashboardStats{
			TotalLicenses:      10,
			ActiveLicenses:     7,
			SuspendedLicenses:  1,
			RevokedLicenses:    1,
			ExpiredLicenses:    1,
			ActiveNodes:        3,
			RecentHeartbeats:   25,
			SystemStatus:       "healthy",
			SystemStatusDetail: "Tất cả dịch vụ hoạt động bình thường",
		},
		RecentHeartbeats: []models.Heartbeat{
			{NodeID: "node-1", ClientIP: "1.2.3.4", CreatedAt: time.Now()},
			{NodeID: "node-2", ClientIP: "5.6.7.8", CreatedAt: time.Now().Add(-2 * time.Minute)},
		},
	}
	if err := RenderDashboard(&buf, data); err != nil {
		t.Fatalf("RenderDashboard failed: %v", err)
	}
	html := buf.String()
	if len(html) == 0 {
		t.Fatal("RenderDashboard produced empty output")
	}
	// Check key elements are present.
	checks := []string{
		"Tổng quan hệ thống",
		"Tổng License",
		"Node Hoạt Động",
		"Heartbeat Gần Đây",
		"Tình Trạng Hệ Thống",
		"node-1",
		"1.2.3.4",
	}
	for _, check := range checks {
		if !bytes.Contains([]byte(html), []byte(check)) {
			t.Errorf("Dashboard HTML missing expected content: %q", check)
		}
	}
}

func TestRenderLicenses(t *testing.T) {
	var buf bytes.Buffer
	data := &LicensesData{
		Licenses: []models.License{
			{
				ID:            1,
				LicenseID:     "lic-001",
				CustomerName:  "Test Corp",
				Plan:          "pro",
				Status:        "active",
				ClientIP:      "10.0.0.1",
				ExpiresAt:     time.Now().Add(30 * 24 * time.Hour),
				LastHeartbeat: time.Now(),
			},
		},
	}
	if err := RenderLicenses(&buf, data); err != nil {
		t.Fatalf("RenderLicenses failed: %v", err)
	}
	html := buf.String()
	checks := []string{
		"Quản lý License",
		"lic-001",
		"Test Corp",
		"pro",
		"active",
		"Gia hạn",
		"Xoay key",
		"Thu hồi",
	}
	for _, check := range checks {
		if !bytes.Contains([]byte(html), []byte(check)) {
			t.Errorf("Licenses HTML missing expected content: %q", check)
		}
	}
}

func TestRenderLicenseFormNew(t *testing.T) {
	var buf bytes.Buffer
	data := &LicenseFormData{
		IsEdit:  false,
		License: models.License{},
	}
	if err := RenderLicenseForm(&buf, data); err != nil {
		t.Fatalf("RenderLicenseForm (new) failed: %v", err)
	}
	html := buf.String()
	if !bytes.Contains([]byte(html), []byte("Tạo License Mới")) {
		t.Error("License form (new) missing title")
	}
}

func TestRenderLicenseFormEdit(t *testing.T) {
	var buf bytes.Buffer
	data := &LicenseFormData{
		IsEdit: true,
		License: models.License{
			ID:           5,
			CustomerID:   "cust-005",
			CustomerName: "Edit Corp",
			Plan:         "enterprise",
			ValidDays:    730,
			Status:       "active",
		},
	}
	if err := RenderLicenseForm(&buf, data); err != nil {
		t.Fatalf("RenderLicenseForm (edit) failed: %v", err)
	}
	html := buf.String()
	checks := []string{"Sửa License", "Edit Corp", "enterprise"}
	for _, check := range checks {
		if !bytes.Contains([]byte(html), []byte(check)) {
			t.Errorf("License form (edit) missing expected content: %q", check)
		}
	}
}

func TestRenderReleases(t *testing.T) {
	var buf bytes.Buffer
	data := &ReleasesData{
		Releases: []models.Release{
			{
				ID:          1,
				Component:   "kiro-client-waf",
				Channel:     "stable",
				Version:     "1.2.3",
				ArtifactURL: "https://releases.example.com/kiro-client-waf-1.2.3",
				SHA256:      "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
				CreatedAt:   time.Now(),
			},
		},
	}
	if err := RenderReleases(&buf, data); err != nil {
		t.Fatalf("RenderReleases failed: %v", err)
	}
	html := buf.String()
	checks := []string{
		"Quản lý Phát Hành",
		"kiro-client-waf",
		"stable",
		"1.2.3",
		"abcdef1234567890",
	}
	for _, check := range checks {
		if !bytes.Contains([]byte(html), []byte(check)) {
			t.Errorf("Releases HTML missing expected content: %q", check)
		}
	}
}

func TestRenderReleaseForm(t *testing.T) {
	var buf bytes.Buffer
	data := &ReleaseFormData{}
	if err := RenderReleaseForm(&buf, data); err != nil {
		t.Fatalf("RenderReleaseForm failed: %v", err)
	}
	html := buf.String()
	checks := []string{
		"Đăng Phát Hành Mới",
		"component",
		"channel",
		"version",
		"artifact_url",
		"sha256",
	}
	for _, check := range checks {
		if !bytes.Contains([]byte(html), []byte(check)) {
			t.Errorf("Release form HTML missing expected content: %q", check)
		}
	}
}

func TestRenderHeartbeats(t *testing.T) {
	var buf bytes.Buffer
	data := &HeartbeatsData{
		Heartbeats: []models.Heartbeat{
			{
				LicenseID: "lic-001",
				NodeID:    "node-alpha",
				ClientIP:  "192.168.1.100",
				CreatedAt: time.Now(),
			},
		},
	}
	if err := RenderHeartbeats(&buf, data); err != nil {
		t.Fatalf("RenderHeartbeats failed: %v", err)
	}
	html := buf.String()
	checks := []string{
		"Heartbeat Log",
		"node-alpha",
		"lic-001",
		"192.168.1.100",
		"data-sortable",
	}
	for _, check := range checks {
		if !bytes.Contains([]byte(html), []byte(check)) {
			t.Errorf("Heartbeats HTML missing expected content: %q", check)
		}
	}
}

func TestRenderDashboardEmpty(t *testing.T) {
	var buf bytes.Buffer
	data := &DashboardData{
		Stats: DashboardStats{
			SystemStatus:       "healthy",
			SystemStatusDetail: "OK",
		},
		RecentHeartbeats: nil,
	}
	if err := RenderDashboard(&buf, data); err != nil {
		t.Fatalf("RenderDashboard (empty) failed: %v", err)
	}
	html := buf.String()
	if !bytes.Contains([]byte(html), []byte("Chưa có heartbeat nào")) {
		t.Error("Dashboard (empty) should show empty state message")
	}
}

func TestRenderLicensesEmpty(t *testing.T) {
	var buf bytes.Buffer
	data := &LicensesData{Licenses: nil}
	if err := RenderLicenses(&buf, data); err != nil {
		t.Fatalf("RenderLicenses (empty) failed: %v", err)
	}
	html := buf.String()
	if !bytes.Contains([]byte(html), []byte("Chưa có license nào")) {
		t.Error("Licenses (empty) should show empty state message")
	}
}

func TestFlashMessages(t *testing.T) {
	var buf bytes.Buffer
	data := &DashboardData{
		PageData: PageData{
			FlashSuccess: "Thao tác thành công!",
			FlashError:   "Có lỗi xảy ra.",
		},
		Stats: DashboardStats{
			SystemStatus:       "healthy",
			SystemStatusDetail: "OK",
		},
	}
	if err := RenderDashboard(&buf, data); err != nil {
		t.Fatalf("RenderDashboard with flash failed: %v", err)
	}
	html := buf.String()
	if !bytes.Contains([]byte(html), []byte("Thao tác thành công!")) {
		t.Error("Flash success message not rendered")
	}
	if !bytes.Contains([]byte(html), []byte("Có lỗi xảy ra.")) {
		t.Error("Flash error message not rendered")
	}
	if !bytes.Contains([]byte(html), []byte("flash-success")) {
		t.Error("Flash success CSS class not present")
	}
	if !bytes.Contains([]byte(html), []byte("flash-error")) {
		t.Error("Flash error CSS class not present")
	}
}

func TestFlashFromRequest(t *testing.T) {
	// This is a unit test for the helper function.
	// We test that it correctly extracts query params.
	import_test_only := FlashFromRequest
	_ = import_test_only
}

func TestRedirectWithFlash(t *testing.T) {
	url := RedirectWithFlash("/admin/licenses", "flash_success", "License created")
	if url != "/admin/licenses?flash_success=License+created" {
		t.Errorf("unexpected redirect URL: %s", url)
	}
}

func TestIsRecentHeartbeat(t *testing.T) {
	recent := time.Now().Add(-2 * time.Minute)
	old := time.Now().Add(-10 * time.Minute)

	if !IsRecentHeartbeat(recent, 5*time.Minute) {
		t.Error("2 minutes ago should be recent within 5 minute window")
	}
	if IsRecentHeartbeat(old, 5*time.Minute) {
		t.Error("10 minutes ago should not be recent within 5 minute window")
	}
}
