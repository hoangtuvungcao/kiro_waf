package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"kiro_waf/master-server/db"
	"kiro_waf/master-server/handlers"
)

func main() {
	// Load configuration from environment variables.
	addr := envOrDefault("KIRO_MASTER_ADDR", ":8080")
	dbPath := envOrDefault("KIRO_MASTER_DB", "/var/lib/kiro-master/master.db")
	adminKey := os.Getenv("KIRO_MASTER_ADMIN_KEY")
	adminIPsRaw := os.Getenv("KIRO_MASTER_ADMIN_IPS")
	sessionTTLStr := envOrDefault("KIRO_MASTER_SESSION_TTL", "12h")

	if adminKey == "" {
		log.Fatal("KIRO_MASTER_ADMIN_KEY is required")
	}

	sessionTTL, err := time.ParseDuration(sessionTTLStr)
	if err != nil {
		log.Fatalf("invalid KIRO_MASTER_SESSION_TTL %q: %v", sessionTTLStr, err)
	}

	// Parse admin IP allowlist.
	var adminIPs []string
	if adminIPsRaw != "" {
		for _, ip := range strings.Split(adminIPsRaw, ",") {
			ip = strings.TrimSpace(ip)
			if ip != "" {
				adminIPs = append(adminIPs, ip)
			}
		}
	}

	// Initialize SQLite database.
	database, err := db.New(dbPath)
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	defer database.Close()

	// Build admin auth config.
	adminConfig := &handlers.AdminAuthConfig{
		AdminKey:   adminKey,
		AllowedIPs: adminIPs,
		SessionTTL: sessionTTL,
	}

	// Register routes.
	mux := http.NewServeMux()

	// Public routes.
	mux.HandleFunc("/", handlers.HandleHomepage())
	mux.HandleFunc("/healthz", handlers.HandleHealthz())

	// API routes.
	mux.HandleFunc("/api/v1/heartbeat", handlers.HandleHeartbeat(database))
	mux.HandleFunc("/api/v1/update/check", handlers.HandleUpdateCheck(database))

	// Admin routes (with IP allowlist, brute-force, and session middleware).
	handlers.RegisterAdminRoutes(mux, database, adminConfig)

	// Apply middleware stack: recovery → logging → mux.
	handler := recoveryMiddleware(loggingMiddleware(mux))

	// Create HTTP server.
	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// Start server in a goroutine.
	go func() {
		log.Printf("kiro-master starting on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown on SIGINT/SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}

	log.Println("server stopped")
}

// envOrDefault returns the value of the environment variable named by key,
// or defaultVal if the variable is not set or empty.
func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// loggingMiddleware logs each request's method, path, status code, and duration.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		duration := time.Since(start)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rw.statusCode, duration)
	})
}

// recoveryMiddleware catches panics in handlers and returns a 500 response.
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic recovered: %v", rec)
				http.Error(w, fmt.Sprintf("internal server error"), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
