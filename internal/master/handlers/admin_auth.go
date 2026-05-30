package handlers

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net/http"
	"time"

	"kiro_waf/internal/master/db"
	"kiro_waf/internal/master/models"
)

// AdminAuthConfig holds configuration for admin authentication.
type AdminAuthConfig struct {
	AdminKey   string        // Shared secret for admin login
	AllowedIPs []string     // List of IPs allowed to access /admin
	SessionTTL time.Duration // Session cookie TTL (default 12h)
}

const (
	sessionCookieName = "kiro_admin_session"
)

// AdminIPAllowlistMiddleware returns an http.Handler that checks if the
// request IP is in the admin allowlist. Returns 404 for disallowed IPs
// to hide the admin endpoint's existence.
func AdminIPAllowlistMiddleware(config *AdminAuthConfig, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := extractClientIP(r)

		if !isIPAllowed(clientIP, config.AllowedIPs) {
			http.NotFound(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// AdminBruteForceMiddleware returns an http.Handler that blocks IPs
// that have exceeded 5 failed login attempts within the last 10 minutes.
// Blocked IPs are rejected for 30 minutes.
func AdminBruteForceMiddleware(database *db.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := extractClientIP(r)
		ctx := r.Context()

		// Check failed attempts in the last 10 minutes.
		since := time.Now().Add(-10 * time.Minute)
		count, err := database.CountFailedAttemptsCtx(ctx, clientIP, since)
		if err != nil {
			log.Printf("admin_auth: brute-force check error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		if count >= 5 {
			// Check if the most recent failed attempt was within the last 30 minutes.
			// If so, the IP is still blocked.
			sinceLock := time.Now().Add(-30 * time.Minute)
			lockCount, err := database.CountFailedAttemptsCtx(ctx, clientIP, sinceLock)
			if err != nil {
				log.Printf("admin_auth: brute-force lock check error: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			if lockCount >= 5 {
				http.Error(w, "too many failed attempts, try again later", http.StatusTooManyRequests)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// AdminSessionMiddleware returns an http.Handler that validates the admin
// session cookie. If no valid session exists, it redirects to the login page.
func AdminSessionMiddleware(database *db.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}

		ctx := r.Context()
		session, err := database.GetSessionByTokenCtx(ctx, cookie.Value)
		if err != nil {
			log.Printf("admin_auth: session lookup error: %v", err)
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}

		if session == nil {
			// Session not found or expired — clear cookie and redirect.
			clearSessionCookie(w)
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// HandleAdminLogin returns an http.HandlerFunc for POST /admin/login.
// It validates the admin key, creates a session, and sets an HttpOnly cookie.
func HandleAdminLogin(database *db.DB, config *AdminAuthConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// Serve login form (simple HTML for now).
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(loginFormHTML))
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse form data.
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		adminKey := r.FormValue("admin_key")
		clientIP := extractClientIP(r)

		// Constant-time comparison to prevent timing attacks.
		keyMatch := subtle.ConstantTimeCompare([]byte(adminKey), []byte(config.AdminKey)) == 1

		if !keyMatch {
			// Log failed attempt.
			if err := database.LogLoginAttemptCtx(r.Context(), clientIP, false); err != nil {
				log.Printf("admin_auth: log failed attempt error: %v", err)
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(loginFailedHTML))
			return
		}

		// Log successful attempt.
		if err := database.LogLoginAttemptCtx(r.Context(), clientIP, true); err != nil {
			log.Printf("admin_auth: log success attempt error: %v", err)
		}

		// Generate session token.
		token, err := generateSessionToken()
		if err != nil {
			log.Printf("admin_auth: generate token error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// Determine session TTL.
		ttl := config.SessionTTL
		if ttl == 0 {
			ttl = 12 * time.Hour
		}

		now := time.Now()
		session := &models.AdminSession{
			SessionToken: token,
			IP:           clientIP,
			CreatedAt:    now,
			ExpiresAt:    now.Add(ttl),
		}

		if err := database.CreateSessionCtx(r.Context(), session); err != nil {
			log.Printf("admin_auth: create session error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// Set session cookie: HttpOnly, SameSite=Strict, Secure, with TTL.
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    token,
			Path:     "/admin",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   true,
			MaxAge:   int(ttl.Seconds()),
		})

		http.Redirect(w, r, "/admin/", http.StatusSeeOther)
	}
}

// HandleAdminLogout returns an http.HandlerFunc for POST /admin/logout.
// It deletes the session from the database and clears the cookie.
func HandleAdminLogout(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		cookie, err := r.Cookie(sessionCookieName)
		if err == nil && cookie.Value != "" {
			if err := database.DeleteSessionCtx(r.Context(), cookie.Value); err != nil {
				log.Printf("admin_auth: delete session error: %v", err)
			}
		}

		clearSessionCookie(w)
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
	}
}

// isIPAllowed checks if the given IP is in the allowlist.
// An empty allowlist means all IPs are allowed (for development).
func isIPAllowed(ip string, allowedIPs []string) bool {
	if len(allowedIPs) == 0 {
		return true
	}
	for _, allowed := range allowedIPs {
		if allowed == ip {
			return true
		}
	}
	return false
}

// generateSessionToken creates a cryptographically secure random token.
func generateSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// clearSessionCookie sets the session cookie to expire immediately.
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/admin",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
		MaxAge:   -1,
	})
}

// loginFormHTML is a minimal login form for the admin panel.
const loginFormHTML = `<!DOCTYPE html>
<html lang="vi">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Kiro WAF - Admin Login</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:system-ui,-apple-system,sans-serif;background:#0f1117;color:#e4e4e7;min-height:100vh;display:flex;align-items:center;justify-content:center}
.login-box{background:#1a1d27;border:1px solid #2d3748;border-radius:12px;padding:2rem;width:100%;max-width:400px}
h1{font-size:1.5rem;margin-bottom:1.5rem;text-align:center;color:#a78bfa}
label{display:block;margin-bottom:0.5rem;font-size:0.875rem;color:#9ca3af}
input[type="password"]{width:100%;padding:0.75rem;border:1px solid #374151;border-radius:8px;background:#0f1117;color:#e4e4e7;font-size:1rem;margin-bottom:1.5rem}
input[type="password"]:focus{outline:none;border-color:#a78bfa}
button{width:100%;padding:0.75rem;border:none;border-radius:8px;background:linear-gradient(135deg,#7c3aed,#a78bfa);color:#fff;font-size:1rem;font-weight:600;cursor:pointer;transition:opacity 0.2s}
button:hover{opacity:0.9}
</style>
</head>
<body>
<div class="login-box">
<h1>🛡️ Kiro WAF Admin</h1>
<form method="POST" action="/admin/login">
<label for="admin_key">Admin Key</label>
<input type="password" id="admin_key" name="admin_key" placeholder="Nhập admin key..." required autofocus>
<button type="submit">Đăng nhập</button>
</form>
</div>
</body>
</html>`

// loginFailedHTML is shown when login fails.
const loginFailedHTML = `<!DOCTYPE html>
<html lang="vi">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Kiro WAF - Đăng nhập thất bại</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:system-ui,-apple-system,sans-serif;background:#0f1117;color:#e4e4e7;min-height:100vh;display:flex;align-items:center;justify-content:center}
.login-box{background:#1a1d27;border:1px solid #2d3748;border-radius:12px;padding:2rem;width:100%;max-width:400px}
h1{font-size:1.5rem;margin-bottom:1rem;text-align:center;color:#a78bfa}
.error{background:#7f1d1d;border:1px solid #991b1b;border-radius:8px;padding:0.75rem;margin-bottom:1.5rem;font-size:0.875rem;color:#fca5a5;text-align:center}
label{display:block;margin-bottom:0.5rem;font-size:0.875rem;color:#9ca3af}
input[type="password"]{width:100%;padding:0.75rem;border:1px solid #374151;border-radius:8px;background:#0f1117;color:#e4e4e7;font-size:1rem;margin-bottom:1.5rem}
input[type="password"]:focus{outline:none;border-color:#a78bfa}
button{width:100%;padding:0.75rem;border:none;border-radius:8px;background:linear-gradient(135deg,#7c3aed,#a78bfa);color:#fff;font-size:1rem;font-weight:600;cursor:pointer;transition:opacity 0.2s}
button:hover{opacity:0.9}
</style>
</head>
<body>
<div class="login-box">
<h1>🛡️ Kiro WAF Admin</h1>
<div class="error">Admin key không đúng. Vui lòng thử lại.</div>
<form method="POST" action="/admin/login">
<label for="admin_key">Admin Key</label>
<input type="password" id="admin_key" name="admin_key" placeholder="Nhập admin key..." required autofocus>
<button type="submit">Đăng nhập</button>
</form>
</div>
</body>
</html>`
