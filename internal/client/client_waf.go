// Package client là entry point cho Client_WAF reverse proxy.
// Khởi tạo tất cả components: cookie, ratelimit, ban, ua, challenge, proxy.
// Start heartbeat loop và update check loop.
// Register routes: challenge, hold, healthz, proxy catch-all.
// Graceful shutdown, fatal on missing config (cookie_secret, license_key).
package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"kiro_waf/internal/client/ban"
	"kiro_waf/internal/client/challenge"
	"kiro_waf/internal/client/cookie"
	"kiro_waf/internal/client/ratelimit"
)

// clientConfig chứa toàn bộ cấu hình cho Client_WAF.
type clientConfig struct {
	ListenAddr       string
	BackendURL       string
	MasterURL        string
	LicenseKey       string
	CookieSecret     string
	NodeID           string
	PoWDifficulty    int
	HoldSeconds      int
	RPMPerIP         int
	SubnetRPM        int
	HardBlockAfter   int
	BlockTTLSeconds  int
	BlocklistFile    string
	XDPSyncCommand   string
	HeartbeatSeconds int
	UpdateSeconds    int
	AdminIPs         []string
	ChallengeAllNew  bool
}

// Run is the entry point for the Client_WAF application.
// Returns 0 on success, 1 on failure.
func Run() int {
	cfg := loadConfig()

	// Fatal on missing required config
	if strings.TrimSpace(cfg.LicenseKey) == "" {
		log.Print("FATAL: KIRO_LICENSE_KEY is required but not set")
		return 1
	}
	if strings.TrimSpace(cfg.CookieSecret) == "" {
		log.Print("FATAL: KIRO_CLIENT_COOKIE_SECRET is required but not set")
		return 1
	}
	if strings.TrimSpace(cfg.BackendURL) == "" {
		log.Print("FATAL: KIRO_BACKEND_URL is required but not set")
		return 1
	}
	if strings.TrimSpace(cfg.MasterURL) == "" {
		log.Print("FATAL: KIRO_MASTER_URL is required but not set")
		return 1
	}

	// Initialize components
	cookieMgr := cookie.NewHMACCookieManager()

	rateLimiter := ratelimit.NewSlidingWindowLimiter(ratelimit.LimiterConfig{
		SoftThreshold:   cfg.RPMPerIP,
		HardThreshold:   cfg.HardBlockAfter,
		SubnetThreshold: cfg.SubnetRPM,
		WindowDuration:  60 * time.Second,
	})

	banEngine := ban.NewInMemoryBanEngine(cfg.BlocklistFile, cfg.XDPSyncCommand)

	challengeStore := challenge.NewStore()

	lockdownMgr := NewLockdownManager(cfg.AdminIPs)

	proxyHandler := NewProxyHandler(
		ProxyConfig{
			BackendURL:      cfg.BackendURL,
			CookieSecret:    []byte(cfg.CookieSecret),
			CookieTTL:       20 * time.Minute,
			Difficulty:      cfg.PoWDifficulty,
			HoldSeconds:     cfg.HoldSeconds,
			BanDuration:     time.Duration(cfg.BlockTTLSeconds) * time.Second,
			ChallengeTTL:    90 * time.Second,
			ChallengeAllNew: cfg.ChallengeAllNew,
		},
		cookieMgr,
		rateLimiter,
		banEngine,
		challengeStore,
	)

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start heartbeat loop
	heartbeatConfig := HeartbeatConfig{
		MasterURL:       cfg.MasterURL,
		LicenseKey:      cfg.LicenseKey,
		NodeID:          cfg.NodeID,
		FingerprintHash: "", // Disabled: binary hash changes on every deploy, causing lockouts
		Interval:        time.Duration(cfg.HeartbeatSeconds) * time.Second,
		Stats:           nil,
	}
	go StartHeartbeatLoop(ctx, heartbeatConfig, lockdownMgr)

	// Start update check loop
	updateConfig := UpdateCheckConfig{
		MasterURL:      cfg.MasterURL,
		Component:      "kiro-client-waf",
		Channel:        "stable",
		CurrentVersion: "0.0.0",
		Interval:       time.Duration(cfg.UpdateSeconds) * time.Second,
	}
	go StartUpdateCheckLoop(ctx, updateConfig)

	// Periodic challenge store cleanup (every 60 seconds)
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				challengeStore.Cleanup()
			}
		}
	}()

	// Periodic rate limiter cleanup (every 120 seconds)
	go func() {
		ticker := time.NewTicker(120 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				rateLimiter.Cleanup()
			}
		}
	}()

	// Register HTTP routes
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if lockdownMgr.IsLocked() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"locked"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Challenge and hold endpoints are handled by ProxyHandler internally
	// via /__kiro/challenge/verify and /__kiro/hold/verify path checks.
	// The catch-all route delegates to ProxyHandler which handles all paths.
	mux.Handle("/", lockdownMiddleware(lockdownMgr, proxyHandler))

	// Create HTTP server
	server := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      securityHeadersMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("kiro client WAF listening on %s backend=%s master=%s node_id=%s",
			cfg.ListenAddr, cfg.BackendURL, cfg.MasterURL, cfg.NodeID)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Graceful shutdown on SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("shutting down gracefully...")
	cancel() // stop background goroutines

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
	log.Println("shutdown complete")
	return 0
}

// loadConfig loads configuration from environment variables with defaults.
func loadConfig() clientConfig {
	adminIPsRaw := strings.TrimSpace(os.Getenv("KIRO_ADMIN_IPS"))
	var adminIPs []string
	if adminIPsRaw != "" {
		for _, ip := range strings.Split(adminIPsRaw, ",") {
			trimmed := strings.TrimSpace(ip)
			if trimmed != "" {
				adminIPs = append(adminIPs, trimmed)
			}
		}
	}

	return clientConfig{
		ListenAddr:       envDefault("KIRO_CLIENT_LISTEN", ":8090"),
		BackendURL:       os.Getenv("KIRO_BACKEND_URL"),
		MasterURL:        os.Getenv("KIRO_MASTER_URL"),
		LicenseKey:       os.Getenv("KIRO_LICENSE_KEY"),
		CookieSecret:     os.Getenv("KIRO_CLIENT_COOKIE_SECRET"),
		NodeID:           envDefault("KIRO_NODE_ID", getHostname()),
		PoWDifficulty:    envInt("KIRO_POW_DIFFICULTY", 4),
		HoldSeconds:      envInt("KIRO_HOLD_SECONDS", 2),
		RPMPerIP:         envInt("KIRO_RPM_PER_IP", 120),
		SubnetRPM:        envInt("KIRO_SUBNET_RPM", 1800),
		HardBlockAfter:   envInt("KIRO_HARD_BLOCK_AFTER", 360),
		BlockTTLSeconds:  envInt("KIRO_BLOCK_TTL_SECONDS", 900),
		BlocklistFile:    envDefault("KIRO_XDP_BLOCKLIST_FILE", "/var/lib/kiro/xdp-blocklist.txt"),
		XDPSyncCommand:   envDefault("KIRO_XDP_SYNC_COMMAND", ""),
		HeartbeatSeconds: envInt("KIRO_HEARTBEAT_SECONDS", 60),
		UpdateSeconds:    envInt("KIRO_UPDATE_SECONDS", 300),
		AdminIPs:         adminIPs,
		ChallengeAllNew:  os.Getenv("KIRO_CHALLENGE_ALL_NEW") == "true" || os.Getenv("KIRO_CHALLENGE_ALL_NEW") == "1",
	}
}

// lockdownMiddleware wraps an http.Handler to block all traffic when in lockdown mode,
// except from admin IPs.
func lockdownMiddleware(lm *LockdownManager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if lm.IsLocked() {
			ip := ClientIP(r)
			if !lm.IsAdminIP(ip) {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte("service temporarily unavailable"))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// securityHeadersMiddleware adds security headers to all responses.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

// envDefault returns the environment variable value or a default.
func envDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// envInt returns the environment variable as int or a default.
func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n := 0
	for _, ch := range v {
		if ch < '0' || ch > '9' {
			return fallback
		}
		n = n*10 + int(ch-'0')
	}
	return n
}

// getHostname returns the system hostname or a default.
func getHostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "kiro-client"
	}
	return h
}

// ClientIP extracts the client IP from the request.
// Checks X-Forwarded-For header first, then falls back to RemoteAddr.
func ClientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		first, _, _ := strings.Cut(forwarded, ",")
		trimmed := strings.TrimSpace(first)
		if trimmed != "" {
			return trimmed
		}
	}
	// RemoteAddr is "host:port"
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx > 0 {
		// Handle IPv6 [::1]:port format
		if addr[0] == '[' {
			bracketEnd := strings.Index(addr, "]")
			if bracketEnd > 0 {
				return addr[1:bracketEnd]
			}
		}
		return addr[:idx]
	}
	return addr
}

// computeBinaryHash computes the SHA-256 hash of the running binary for integrity verification.
// Returns empty string if the binary cannot be read (non-fatal).
func computeBinaryHash() string {
	execPath, err := os.Executable()
	if err != nil {
		log.Printf("WARNING: cannot determine executable path for integrity check: %v", err)
		return ""
	}
	f, err := os.Open(execPath)
	if err != nil {
		log.Printf("WARNING: cannot open executable for integrity check: %v", err)
		return ""
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Printf("WARNING: cannot hash executable for integrity check: %v", err)
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}
