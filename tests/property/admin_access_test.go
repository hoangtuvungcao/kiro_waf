// Feature: waf-system-overhaul, Property 7: Admin IP Access Control
// **Validates: Requirements 3.2**
//
// For any IP address không nằm trong danh sách admin allowlist, yêu cầu đến
// đường dẫn `/admin/` SHALL trả về HTTP 404. For any IP trong allowlist, yêu cầu
// SHALL trả về form đăng nhập (200) hoặc dashboard (nếu có session hợp lệ).
package property

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"kiro_waf/master-server/handlers"

	"pgregory.net/rapid"
)

// dummyHandler is a simple handler that returns 200 OK, simulating the next
// handler in the chain (login form or dashboard).
var dummyHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("admin content"))
})

// generateIPv4 generates a random valid IPv4 address string.
func generateIPv4(t *rapid.T, label string) string {
	return fmt.Sprintf("%d.%d.%d.%d",
		rapid.IntRange(1, 254).Draw(t, label+"_o1"),
		rapid.IntRange(0, 255).Draw(t, label+"_o2"),
		rapid.IntRange(0, 255).Draw(t, label+"_o3"),
		rapid.IntRange(1, 254).Draw(t, label+"_o4"),
	)
}

// generateAllowlist generates a random allowlist of 1-10 IP addresses.
func generateAllowlist(t *rapid.T) []string {
	count := rapid.IntRange(1, 10).Draw(t, "allowlist_size")
	ips := make([]string, count)
	for i := 0; i < count; i++ {
		ips[i] = generateIPv4(t, fmt.Sprintf("allow_%d", i))
	}
	return ips
}

// isInList checks if an IP is in the given list.
func isInList(ip string, list []string) bool {
	for _, item := range list {
		if item == ip {
			return true
		}
	}
	return false
}

// TestAdminAccess_DeniedIP verifies that for any IP NOT in the allowlist,
// requests to /admin/ return HTTP 404.
func TestAdminAccess_DeniedIP(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random allowlist
		allowlist := generateAllowlist(t)

		// Generate a random IP that is NOT in the allowlist
		var clientIP string
		for {
			clientIP = generateIPv4(t, "client")
			if !isInList(clientIP, allowlist) {
				break
			}
			// Extremely unlikely to collide, but regenerate if it does
			clientIP = generateIPv4(t, "client_retry")
			if !isInList(clientIP, allowlist) {
				break
			}
		}

		// Create the middleware with the allowlist config
		config := &handlers.AdminAuthConfig{
			AdminKey:   "test-admin-key",
			AllowedIPs: allowlist,
		}

		middleware := handlers.AdminIPAllowlistMiddleware(config, dummyHandler)

		// Create request from the denied IP
		req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
		req.RemoteAddr = clientIP + ":12345"
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		// Property: IP NOT in allowlist SHALL receive 404
		if rec.Code != http.StatusNotFound {
			t.Fatalf("Expected 404 for IP %s not in allowlist %v, got %d",
				clientIP, allowlist, rec.Code)
		}
	})
}

// TestAdminAccess_AllowedIP verifies that for any IP IN the allowlist,
// requests to /admin/ are passed through to the next handler (200).
func TestAdminAccess_AllowedIP(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random allowlist
		allowlist := generateAllowlist(t)

		// Pick a random IP from the allowlist
		idx := rapid.IntRange(0, len(allowlist)-1).Draw(t, "allowed_idx")
		clientIP := allowlist[idx]

		// Create the middleware with the allowlist config
		config := &handlers.AdminAuthConfig{
			AdminKey:   "test-admin-key",
			AllowedIPs: allowlist,
		}

		middleware := handlers.AdminIPAllowlistMiddleware(config, dummyHandler)

		// Create request from the allowed IP
		req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
		req.RemoteAddr = clientIP + ":12345"
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		// Property: IP IN allowlist SHALL receive 200 (pass through to next handler)
		if rec.Code != http.StatusOK {
			t.Fatalf("Expected 200 for IP %s in allowlist %v, got %d",
				clientIP, allowlist, rec.Code)
		}
	})
}

// TestAdminAccess_EmptyAllowlistAllowsAll verifies that when the allowlist is
// empty, all IPs are allowed (development mode behavior).
func TestAdminAccess_EmptyAllowlistAllowsAll(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random IP
		clientIP := generateIPv4(t, "client")

		// Create the middleware with an empty allowlist
		config := &handlers.AdminAuthConfig{
			AdminKey:   "test-admin-key",
			AllowedIPs: []string{},
		}

		middleware := handlers.AdminIPAllowlistMiddleware(config, dummyHandler)

		// Create request from any IP
		req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
		req.RemoteAddr = clientIP + ":12345"
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		// Property: Empty allowlist means all IPs are allowed (dev mode)
		if rec.Code != http.StatusOK {
			t.Fatalf("Expected 200 for IP %s with empty allowlist (dev mode), got %d",
				clientIP, rec.Code)
		}
	})
}
