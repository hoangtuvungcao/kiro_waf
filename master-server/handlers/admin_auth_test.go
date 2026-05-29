package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"kiro_waf/master-server/db"
	"kiro_waf/master-server/models"
)

// setupTestDB creates a temporary SQLite database for testing.
func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "admin_auth_test_*.db")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	database, err := db.New(tmpFile.Name())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestAdminIPAllowlistMiddleware_AllowedIP(t *testing.T) {
	config := &AdminAuthConfig{
		AllowedIPs: []string{"192.168.1.1", "10.0.0.1"},
	}

	handler := AdminIPAllowlistMiddleware(config, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAdminIPAllowlistMiddleware_DeniedIP(t *testing.T) {
	config := &AdminAuthConfig{
		AllowedIPs: []string{"192.168.1.1", "10.0.0.1"},
	}

	handler := AdminIPAllowlistMiddleware(config, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for denied IP")
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	req.RemoteAddr = "1.2.3.4:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestAdminIPAllowlistMiddleware_EmptyAllowlist(t *testing.T) {
	config := &AdminAuthConfig{
		AllowedIPs: []string{},
	}

	handler := AdminIPAllowlistMiddleware(config, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	req.RemoteAddr = "1.2.3.4:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 (empty allowlist allows all), got %d", rec.Code)
	}
}

func TestAdminBruteForceMiddleware_AllowsUnderThreshold(t *testing.T) {
	database := setupTestDB(t)

	// Log 4 failed attempts (under threshold of 5).
	for i := 0; i < 4; i++ {
		if err := database.LogLoginAttempt("10.0.0.1", false); err != nil {
			t.Fatalf("log attempt: %v", err)
		}
	}

	handler := AdminBruteForceMiddleware(database, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/admin/login", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 (under threshold), got %d", rec.Code)
	}
}

func TestAdminBruteForceMiddleware_BlocksAfterThreshold(t *testing.T) {
	database := setupTestDB(t)

	// Log 5 failed attempts (at threshold).
	for i := 0; i < 5; i++ {
		if err := database.LogLoginAttempt("10.0.0.1", false); err != nil {
			t.Fatalf("log attempt: %v", err)
		}
	}

	handler := AdminBruteForceMiddleware(database, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for blocked IP")
	}))

	req := httptest.NewRequest(http.MethodPost, "/admin/login", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}
}

func TestAdminBruteForceMiddleware_DifferentIPNotBlocked(t *testing.T) {
	database := setupTestDB(t)

	// Log 5 failed attempts from one IP.
	for i := 0; i < 5; i++ {
		if err := database.LogLoginAttempt("10.0.0.1", false); err != nil {
			t.Fatalf("log attempt: %v", err)
		}
	}

	handler := AdminBruteForceMiddleware(database, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Different IP should not be blocked.
	req := httptest.NewRequest(http.MethodPost, "/admin/login", nil)
	req.RemoteAddr = "10.0.0.2:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 (different IP), got %d", rec.Code)
	}
}

func TestAdminSessionMiddleware_NoSession(t *testing.T) {
	database := setupTestDB(t)

	handler := AdminSessionMiddleware(database, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without session")
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin/login" {
		t.Errorf("expected redirect to /admin/login, got %s", loc)
	}
}

func TestAdminSessionMiddleware_ValidSession(t *testing.T) {
	database := setupTestDB(t)

	// Create a valid session.
	token := "test-session-token-12345"
	session := &models.AdminSession{
		SessionToken: token,
		IP:           "10.0.0.1",
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(12 * time.Hour),
	}
	if err := database.CreateSession(session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	handler := AdminSessionMiddleware(database, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("dashboard"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestHandleAdminLogin_GET_ShowsForm(t *testing.T) {
	database := setupTestDB(t)
	config := &AdminAuthConfig{
		AdminKey:   "test-secret-key",
		AllowedIPs: []string{},
		SessionTTL: 12 * time.Hour,
	}

	handler := HandleAdminLogin(database, config)

	req := httptest.NewRequest(http.MethodGet, "/admin/login", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "admin_key") {
		t.Error("expected login form with admin_key field")
	}
}

func TestHandleAdminLogin_POST_ValidKey(t *testing.T) {
	database := setupTestDB(t)
	config := &AdminAuthConfig{
		AdminKey:   "test-secret-key",
		AllowedIPs: []string{},
		SessionTTL: 12 * time.Hour,
	}

	handler := HandleAdminLogin(database, config)

	form := url.Values{}
	form.Set("admin_key", "test-secret-key")
	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "10.0.0.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin/" {
		t.Errorf("expected redirect to /admin/, got %s", loc)
	}

	// Check that session cookie was set.
	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == sessionCookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected session cookie to be set")
	}
	if !sessionCookie.HttpOnly {
		t.Error("expected HttpOnly cookie")
	}
	if sessionCookie.SameSite != http.SameSiteStrictMode {
		t.Error("expected SameSite=Strict cookie")
	}
	if sessionCookie.MaxAge != int((12 * time.Hour).Seconds()) {
		t.Errorf("expected MaxAge %d, got %d", int((12*time.Hour).Seconds()), sessionCookie.MaxAge)
	}
}

func TestHandleAdminLogin_POST_InvalidKey(t *testing.T) {
	database := setupTestDB(t)
	config := &AdminAuthConfig{
		AdminKey:   "test-secret-key",
		AllowedIPs: []string{},
		SessionTTL: 12 * time.Hour,
	}

	handler := HandleAdminLogin(database, config)

	form := url.Values{}
	form.Set("admin_key", "wrong-key")
	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "10.0.0.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}

	// Verify failed attempt was logged.
	count, err := database.CountFailedAttempts("10.0.0.1", time.Now().Add(-1*time.Minute))
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 failed attempt logged, got %d", count)
	}
}

func TestHandleAdminLogout(t *testing.T) {
	database := setupTestDB(t)

	handler := HandleAdminLogout(database)

	req := httptest.NewRequest(http.MethodPost, "/admin/logout", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "some-token"})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rec.Code)
	}

	// Check cookie is cleared.
	cookies := rec.Result().Cookies()
	for _, c := range cookies {
		if c.Name == sessionCookieName && c.MaxAge != -1 {
			t.Error("expected session cookie to be cleared (MaxAge=-1)")
		}
	}
}

func TestIsIPAllowed(t *testing.T) {
	tests := []struct {
		name       string
		ip         string
		allowedIPs []string
		want       bool
	}{
		{"empty allowlist allows all", "1.2.3.4", nil, true},
		{"ip in list", "10.0.0.1", []string{"10.0.0.1", "10.0.0.2"}, true},
		{"ip not in list", "10.0.0.3", []string{"10.0.0.1", "10.0.0.2"}, false},
		{"exact match required", "10.0.0.10", []string{"10.0.0.1"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isIPAllowed(tt.ip, tt.allowedIPs)
			if got != tt.want {
				t.Errorf("isIPAllowed(%q, %v) = %v, want %v", tt.ip, tt.allowedIPs, got, tt.want)
			}
		})
	}
}
