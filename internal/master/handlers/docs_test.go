package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDocsHandler_ServesDocsPage(t *testing.T) {
	handler := NewDocsHandler()
	h := handler.HandleDocs()

	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()

	// Should contain Vietnamese content by default.
	if !strings.Contains(body, "Tài Liệu Kiro WAF") {
		t.Error("expected Vietnamese title in default docs page")
	}

	// Should contain sidebar navigation.
	if !strings.Contains(body, "docs-sidebar") {
		t.Error("expected sidebar navigation in docs page")
	}

	// Should contain language switcher.
	if !strings.Contains(body, "lang-switcher") {
		t.Error("expected language switcher in docs page")
	}

	// Should contain both language options.
	if !strings.Contains(body, "Tiếng Việt") {
		t.Error("expected Vietnamese language option")
	}
	if !strings.Contains(body, "English") {
		t.Error("expected English language option")
	}

	// Content-Type should be HTML.
	ct := rr.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Errorf("expected Content-Type text/html; charset=utf-8, got %s", ct)
	}
}

func TestDocsHandler_EnglishLanguage(t *testing.T) {
	handler := NewDocsHandler()
	h := handler.HandleDocs()

	req := httptest.NewRequest(http.MethodGet, "/docs/en", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()

	// Should contain English content.
	if !strings.Contains(body, "Kiro WAF Documentation") {
		t.Error("expected English title in docs page")
	}

	// Should contain English navigation sections.
	if !strings.Contains(body, "Getting Started") {
		t.Error("expected English navigation sections")
	}
	if !strings.Contains(body, "Configuration") {
		t.Error("expected Configuration section")
	}
	if !strings.Contains(body, "Troubleshooting") {
		t.Error("expected Troubleshooting section")
	}
}

func TestDocsHandler_VietnameseLanguage(t *testing.T) {
	handler := NewDocsHandler()
	h := handler.HandleDocs()

	req := httptest.NewRequest(http.MethodGet, "/docs/vi", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()

	// Should contain Vietnamese content.
	if !strings.Contains(body, "Tài Liệu Kiro WAF") {
		t.Error("expected Vietnamese title in docs page")
	}

	// Should contain Vietnamese navigation sections.
	if !strings.Contains(body, "Bắt Đầu") {
		t.Error("expected Vietnamese navigation sections")
	}
	if !strings.Contains(body, "Cấu Hình") {
		t.Error("expected Vietnamese Configuration section")
	}
}

func TestDocsHandler_LanguageSwitcherOnEveryPage(t *testing.T) {
	handler := NewDocsHandler()
	h := handler.HandleDocs()

	paths := []string{"/docs", "/docs/", "/docs/vi", "/docs/en", "/docs/vi/quick-start", "/docs/en/installation"}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()

		h.ServeHTTP(rr, req)

		body := rr.Body.String()
		if !strings.Contains(body, "lang-switcher") {
			t.Errorf("expected language switcher on path %s", path)
		}
	}
}

func TestDocsHandler_SidebarNavigation(t *testing.T) {
	handler := NewDocsHandler()
	h := handler.HandleDocs()

	req := httptest.NewRequest(http.MethodGet, "/docs/en", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	body := rr.Body.String()

	// Should contain sidebar with navigation items.
	if !strings.Contains(body, "docs-nav-section") {
		t.Error("expected navigation sections in sidebar")
	}
	if !strings.Contains(body, "docs-nav-item") {
		t.Error("expected navigation items in sidebar")
	}

	// Should contain links to doc pages.
	if !strings.Contains(body, "/docs/en/quick-start") {
		t.Error("expected quick-start link in sidebar")
	}
	if !strings.Contains(body, "/docs/en/installation") {
		t.Error("expected installation link in sidebar")
	}
	if !strings.Contains(body, "/docs/en/faq") {
		t.Error("expected FAQ link in sidebar")
	}
}

func TestDocsHandler_CustomErrorPage(t *testing.T) {
	handler := NewDocsHandler()
	h := handler.HandleDocsUnavailable()

	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}

	body := rr.Body.String()

	// Should show custom error message, not generic server error.
	if !strings.Contains(body, "tạm thời không khả dụng") {
		t.Error("expected custom Vietnamese error message")
	}

	// Should still have language switcher.
	if !strings.Contains(body, "lang-switcher") {
		t.Error("expected language switcher on error page")
	}
}

func TestDocsHandler_CustomErrorPageEnglish(t *testing.T) {
	handler := NewDocsHandler()
	h := handler.HandleDocsUnavailable()

	req := httptest.NewRequest(http.MethodGet, "/docs/en", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}

	body := rr.Body.String()

	// Should show English custom error message.
	if !strings.Contains(body, "temporarily unavailable") {
		t.Error("expected custom English error message")
	}
}

func TestDocsHandler_MethodNotAllowed(t *testing.T) {
	handler := NewDocsHandler()
	h := handler.HandleDocs()

	req := httptest.NewRequest(http.MethodPost, "/docs", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rr.Code)
	}
}

func TestDocsHandler_ParseLang(t *testing.T) {
	handler := NewDocsHandler()

	tests := []struct {
		path     string
		expected string
	}{
		{"/docs", "vi"},
		{"/docs/", "vi"},
		{"/docs/vi", "vi"},
		{"/docs/en", "en"},
		{"/docs/vi/quick-start", "vi"},
		{"/docs/en/installation", "en"},
		{"/docs/unknown", "vi"},       // Unknown lang defaults to vi.
		{"/docs/fr/something", "vi"},  // Unsupported lang defaults to vi.
	}

	for _, tt := range tests {
		got := handler.parseLang(tt.path)
		if got != tt.expected {
			t.Errorf("parseLang(%q) = %q, want %q", tt.path, got, tt.expected)
		}
	}
}
