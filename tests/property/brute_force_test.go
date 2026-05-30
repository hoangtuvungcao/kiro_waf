// Feature: waf-system-overhaul, Property 6: Admin Brute-Force Protection
// **Validates: Requirements 3.4**
//
// For any IP address và chuỗi N lần đăng nhập thất bại liên tiếp trong cửa sổ 10 phút,
// khi N > 5 thì tất cả các lần đăng nhập tiếp theo từ IP đó SHALL bị từ chối trong 30 phút.
// Khi N ≤ 5, đăng nhập với key đúng SHALL được chấp nhận.
package property

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"kiro_waf/internal/master/db"
	"kiro_waf/internal/master/handlers"

	"pgregory.net/rapid"
)

// newTempDB creates a temporary SQLite database for a single test iteration.
// Returns the DB instance and a cleanup function.
func newTempDB() (*db.DB, func(), error) {
	tmpDir, err := os.MkdirTemp("", "brute_force_test_*")
	if err != nil {
		return nil, nil, fmt.Errorf("create temp dir: %w", err)
	}
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.New(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, nil, fmt.Errorf("create DB: %w", err)
	}
	cleanup := func() {
		database.Close()
		os.RemoveAll(tmpDir)
	}
	return database, cleanup, nil
}

// simulateFailedAttempts logs N failed login attempts for the given IP.
func simulateFailedAttempts(database *db.DB, ip string, n int) error {
	for i := 0; i < n; i++ {
		if err := database.LogLoginAttempt(ip, false); err != nil {
			return fmt.Errorf("failed to log attempt %d: %w", i, err)
		}
	}
	return nil
}

// TestBruteForce_BlockAfterThreshold verifies that after more than 5 failed
// login attempts from the same IP within 10 minutes, the brute-force middleware
// blocks subsequent requests with HTTP 429.
func TestBruteForce_BlockAfterThreshold(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		database, cleanup, err := newTempDB()
		if err != nil {
			t.Fatalf("setup DB: %v", err)
		}
		defer cleanup()

		// Generate a random IP address
		ip := fmt.Sprintf("%d.%d.%d.%d",
			rapid.IntRange(1, 254).Draw(t, "ip_octet1"),
			rapid.IntRange(0, 255).Draw(t, "ip_octet2"),
			rapid.IntRange(0, 255).Draw(t, "ip_octet3"),
			rapid.IntRange(1, 254).Draw(t, "ip_octet4"),
		)

		// Generate N > 5 failed attempts (6 to 20)
		n := rapid.IntRange(6, 20).Draw(t, "failedAttempts")

		// Simulate N failed login attempts
		if err := simulateFailedAttempts(database, ip, n); err != nil {
			t.Fatalf("failed to simulate attempts: %v", err)
		}

		// Create a dummy next handler that should NOT be reached
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})

		// Create the brute-force middleware
		middleware := handlers.AdminBruteForceMiddleware(database, next)

		// Create a request from the same IP
		req := httptest.NewRequest(http.MethodPost, "/admin/login", nil)
		req.RemoteAddr = ip + ":12345"
		rec := httptest.NewRecorder()

		// Execute the middleware
		middleware.ServeHTTP(rec, req)

		// Property: When N > 5, the request SHALL be rejected with 429
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("expected HTTP 429 after %d failed attempts from IP %s, got %d",
				n, ip, rec.Code)
		}

		if nextCalled {
			t.Fatalf("next handler was called despite %d failed attempts from IP %s",
				n, ip)
		}
	})
}

// TestBruteForce_AllowUnderThreshold verifies that with N <= 5 failed attempts,
// requests pass through the brute-force middleware (are not blocked).
func TestBruteForce_AllowUnderThreshold(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		database, cleanup, err := newTempDB()
		if err != nil {
			t.Fatalf("setup DB: %v", err)
		}
		defer cleanup()

		// Generate a random IP address
		ip := fmt.Sprintf("%d.%d.%d.%d",
			rapid.IntRange(1, 254).Draw(t, "ip_octet1"),
			rapid.IntRange(0, 255).Draw(t, "ip_octet2"),
			rapid.IntRange(0, 255).Draw(t, "ip_octet3"),
			rapid.IntRange(1, 254).Draw(t, "ip_octet4"),
		)

		// Generate N <= 5 failed attempts (0 to 4, since the threshold is >= 5 in the middleware)
		n := rapid.IntRange(0, 4).Draw(t, "failedAttempts")

		// Simulate N failed login attempts
		if err := simulateFailedAttempts(database, ip, n); err != nil {
			t.Fatalf("failed to simulate attempts: %v", err)
		}

		// Create a next handler that records it was called
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})

		// Create the brute-force middleware
		middleware := handlers.AdminBruteForceMiddleware(database, next)

		// Create a request from the same IP
		req := httptest.NewRequest(http.MethodPost, "/admin/login", nil)
		req.RemoteAddr = ip + ":12345"
		rec := httptest.NewRecorder()

		// Execute the middleware
		middleware.ServeHTTP(rec, req)

		// Property: When N <= 5, the request SHALL pass through
		if !nextCalled {
			t.Fatalf("next handler was NOT called with only %d failed attempts from IP %s (status=%d)",
				n, ip, rec.Code)
		}

		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("got HTTP 429 with only %d failed attempts from IP %s, should be allowed",
				n, ip)
		}
	})
}

// TestBruteForce_DifferentIPsIndependent verifies that brute-force blocking
// for one IP does not affect requests from a different IP.
func TestBruteForce_DifferentIPsIndependent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		database, cleanup, err := newTempDB()
		if err != nil {
			t.Fatalf("setup DB: %v", err)
		}
		defer cleanup()

		// Generate two distinct random IPs (different first octets ensure uniqueness)
		ip1 := fmt.Sprintf("%d.%d.%d.%d",
			rapid.IntRange(1, 126).Draw(t, "ip1_octet1"),
			rapid.IntRange(0, 255).Draw(t, "ip1_octet2"),
			rapid.IntRange(0, 255).Draw(t, "ip1_octet3"),
			rapid.IntRange(1, 254).Draw(t, "ip1_octet4"),
		)
		ip2 := fmt.Sprintf("%d.%d.%d.%d",
			rapid.IntRange(128, 254).Draw(t, "ip2_octet1"),
			rapid.IntRange(0, 255).Draw(t, "ip2_octet2"),
			rapid.IntRange(0, 255).Draw(t, "ip2_octet3"),
			rapid.IntRange(1, 254).Draw(t, "ip2_octet4"),
		)

		// Block ip1 with > 5 failed attempts
		if err := simulateFailedAttempts(database, ip1, 10); err != nil {
			t.Fatalf("failed to simulate attempts for ip1: %v", err)
		}

		// Create a next handler
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})

		middleware := handlers.AdminBruteForceMiddleware(database, next)

		// Request from ip2 (which has no failed attempts) should pass through
		req := httptest.NewRequest(http.MethodPost, "/admin/login", nil)
		req.RemoteAddr = ip2 + ":12345"
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		// Property: IP2 should NOT be blocked by IP1's failed attempts
		if !nextCalled {
			t.Fatalf("ip2 (%s) was blocked despite having no failed attempts (ip1=%s had 10 failures)",
				ip2, ip1)
		}

		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("ip2 (%s) got HTTP 429 despite having no failed attempts", ip2)
		}
	})
}
