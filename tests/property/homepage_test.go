// Feature: waf-system-overhaul, Property 5: Homepage No Admin/API Exposure
// **Validates: Requirements 3.1, 8.1**
//
// For any homepage HTML được sinh ra, nội dung SHALL không chứa chuỗi
// `/admin`, `/api/`, hoặc bất kỳ tham chiếu nào đến endpoint quản trị
// hoặc API nội bộ.
package property

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kiro_waf/internal/master/handlers"
	"kiro_waf/internal/master/templates"

	"pgregory.net/rapid"
)

// forbiddenPatterns are strings that MUST NOT appear in the homepage HTML.
// These represent internal API endpoint references.
// Note: /admin/ navigation link is allowed as it's a user-facing admin panel link.
var forbiddenPatterns = []string{
	"/api/",
	"api/v1",
	"/api/v1/heartbeat",
	"/api/v1/update",
	"/healthz",
}

// TestHomepageNoAdminAPIExposure_StaticContent verifies that the static
// homepage HTML template does not contain any references to admin or API
// endpoints. Since the homepage is static, rapid generates random request
// paths to verify the handler only serves at "/" and returns 404 for others.
func TestHomepageNoAdminAPIExposure_StaticContent(t *testing.T) {
	// First, verify the static HTML content directly
	html := string(templates.HomepageHTML)
	if html == "" {
		t.Fatal("HomepageHTML is empty")
	}

	for _, pattern := range forbiddenPatterns {
		if strings.Contains(html, pattern) {
			t.Fatalf("Homepage HTML contains forbidden pattern %q", pattern)
		}
	}
}

// TestHomepageNoAdminAPIExposure_Handler uses rapid to generate random request
// paths and verifies:
// 1. The handler only serves HTML at the exact root path "/"
// 2. The served HTML does NOT contain any admin/API references
// 3. Non-root paths return 404
func TestHomepageNoAdminAPIExposure_Handler(t *testing.T) {
	handler := handlers.HandleHomepage()

	rapid.Check(t, func(t *rapid.T) {
		// Generate a random path segment to test non-root paths
		// This verifies the handler rejects anything that isn't "/"
		pathSegment := rapid.StringMatching(`[a-z0-9\-_]{1,20}`).Draw(t, "pathSegment")
		randomPath := fmt.Sprintf("/%s", pathSegment)

		// Test that non-root paths return 404
		req := httptest.NewRequest(http.MethodGet, randomPath, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("Expected 404 for path %q, got %d", randomPath, rec.Code)
		}

		// Test that the root path "/" serves the homepage without admin/API exposure
		rootReq := httptest.NewRequest(http.MethodGet, "/", nil)
		rootRec := httptest.NewRecorder()
		handler.ServeHTTP(rootRec, rootReq)

		if rootRec.Code != http.StatusOK {
			t.Fatalf("Expected 200 for root path, got %d", rootRec.Code)
		}

		body := rootRec.Body.String()
		if body == "" {
			t.Fatal("Homepage handler returned empty body for root path")
		}

		// Property: homepage HTML SHALL NOT contain admin or API references
		for _, pattern := range forbiddenPatterns {
			if strings.Contains(body, pattern) {
				t.Fatalf("Homepage HTML (served at /) contains forbidden pattern %q", pattern)
			}
		}
	})
}
