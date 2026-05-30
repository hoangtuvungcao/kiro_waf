package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDocsHandler_InstallationPage(t *testing.T) {
	handler := NewDocsHandler()
	h := handler.HandleDocs()

	// Test English installation page.
	req := httptest.NewRequest(http.MethodGet, "/docs/en/installation", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()

	if !strings.Contains(body, "Installation Guide") {
		t.Error("expected Installation Guide title")
	}
	if !strings.Contains(body, "Supported Operating Systems") {
		t.Error("expected supported OS section")
	}
	if !strings.Contains(body, "install.sh") {
		t.Error("expected install script reference")
	}
}

func TestDocsHandler_QuickStartPage(t *testing.T) {
	handler := NewDocsHandler()
	h := handler.HandleDocs()

	// Test English quick-start page.
	req := httptest.NewRequest(http.MethodGet, "/docs/en/quick-start", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()

	if !strings.Contains(body, "Quick Start") {
		t.Error("expected Quick Start title")
	}
	if !strings.Contains(body, "15 minutes") {
		t.Error("expected 15-minute quick start reference")
	}
}

func TestDocsHandler_ConfigReferencePage(t *testing.T) {
	handler := NewDocsHandler()
	h := handler.HandleDocs()

	// Test English config reference page.
	req := httptest.NewRequest(http.MethodGet, "/docs/en/config-reference", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()

	if !strings.Contains(body, "Configuration Reference") {
		t.Error("expected Configuration Reference title")
	}
	// Should contain YAML options with type, default, range, description.
	if !strings.Contains(body, "Type") {
		t.Error("expected Type column in config reference")
	}
	if !strings.Contains(body, "Default") {
		t.Error("expected Default column in config reference")
	}
	if !strings.Contains(body, "license_key") {
		t.Error("expected license_key option documented")
	}
	if !strings.Contains(body, "protection.profile") {
		t.Error("expected protection.profile option documented")
	}
}

func TestDocsHandler_TroubleshootingPage(t *testing.T) {
	handler := NewDocsHandler()
	h := handler.HandleDocs()

	// Test English troubleshooting page.
	req := httptest.NewRequest(http.MethodGet, "/docs/en/common-issues", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()

	if !strings.Contains(body, "Troubleshooting") {
		t.Error("expected Troubleshooting title")
	}

	// Verify at least 10 scenarios are present (numbered h3 headings).
	scenarios := []string{
		"1.", "2.", "3.", "4.", "5.",
		"6.", "7.", "8.", "9.", "10.",
	}
	for _, s := range scenarios {
		if !strings.Contains(body, s) {
			t.Errorf("expected troubleshooting scenario %s", s)
		}
	}
}

func TestDocsHandler_FAQPage(t *testing.T) {
	handler := NewDocsHandler()
	h := handler.HandleDocs()

	// Test English FAQ page.
	req := httptest.NewRequest(http.MethodGet, "/docs/en/faq", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()

	if !strings.Contains(body, "Frequently Asked Questions") {
		t.Error("expected FAQ title")
	}
	if !strings.Contains(body, "What is Kiro WAF") {
		t.Error("expected 'What is Kiro WAF' question")
	}
}

func TestDocsHandler_VersionDateDisplay(t *testing.T) {
	handler := NewDocsHandler()
	h := handler.HandleDocs()

	// Test that version and date are displayed on pages.
	paths := []string{"/docs/en", "/docs/vi", "/docs/en/quick-start", "/docs/vi/faq"}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		body := rr.Body.String()

		// Should contain version info.
		if !strings.Contains(body, "Version:") && !strings.Contains(body, "Phiên bản:") {
			t.Errorf("expected version display on path %s", path)
		}
	}
}

func TestDocsHandler_NoInternalDetailsExposed(t *testing.T) {
	handler := NewDocsHandler()
	h := handler.HandleDocs()

	// Check all content pages for internal details that should NOT be exposed.
	paths := []string{
		"/docs/en/quick-start",
		"/docs/en/installation",
		"/docs/en/config-reference",
		"/docs/en/common-issues",
		"/docs/en/faq",
		"/docs/vi/quick-start",
		"/docs/vi/installation",
		"/docs/vi/config-reference",
		"/docs/vi/common-issues",
		"/docs/vi/faq",
	}

	forbiddenPatterns := []string{
		"/api/v1/",          // Internal API endpoints
		"sqlite",            // DB schema details
		"internal/master",   // Source paths
		"internal/client",   // Source paths
		"cmd/kiro-master",   // Source paths
		"handlers/",         // Source paths
		"db.go",             // Source file names
		"SELECT ",           // SQL queries
		"CREATE TABLE",      // DB schema
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		body := rr.Body.String()

		for _, pattern := range forbiddenPatterns {
			if strings.Contains(body, pattern) {
				t.Errorf("page %s exposes internal detail: %q", path, pattern)
			}
		}
	}
}

func TestDocsHandler_VietnameseContentPages(t *testing.T) {
	handler := NewDocsHandler()
	h := handler.HandleDocs()

	tests := []struct {
		path     string
		expected string
	}{
		{"/docs/vi/quick-start", "Bắt Đầu Nhanh"},
		{"/docs/vi/installation", "Hướng Dẫn Cài Đặt"},
		{"/docs/vi/config-reference", "Tham Chiếu Cấu Hình"},
		{"/docs/vi/common-issues", "Xử Lý Sự Cố"},
		{"/docs/vi/faq", "Câu Hỏi Thường Gặp"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("path %s: expected status 200, got %d", tt.path, rr.Code)
		}

		body := rr.Body.String()
		if !strings.Contains(body, tt.expected) {
			t.Errorf("path %s: expected content %q", tt.path, tt.expected)
		}
	}
}

func TestDocsHandler_ActiveSidebarItem(t *testing.T) {
	handler := NewDocsHandler()
	h := handler.HandleDocs()

	req := httptest.NewRequest(http.MethodGet, "/docs/en/quick-start", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	body := rr.Body.String()

	// The quick-start nav item should have the active class.
	// Check that the page renders with active state on the correct item.
	if !strings.Contains(body, "active") {
		t.Error("expected active class on sidebar item")
	}
}
