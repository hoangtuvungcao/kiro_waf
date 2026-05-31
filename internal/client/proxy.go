// Package client implements the main reverse proxy handler for Client_WAF.
// ProxyHandler integrates all components: cookie, ratelimit, ban, ua, challenge,
// escalation, TLS fingerprint, Cloudflare IP extraction, loop detection.
package client

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"kiro_waf/internal/client/ban"
	"kiro_waf/internal/client/cf"
	"kiro_waf/internal/client/challenge"
	"kiro_waf/internal/client/cookie"
	"kiro_waf/internal/client/escalation"
	"kiro_waf/internal/client/fingerprint"
	"kiro_waf/internal/client/ratelimit"
	"kiro_waf/internal/client/ua"
)

// ProxyConfig contains configuration for ProxyHandler.
type ProxyConfig struct {
	BackendURL      string
	CookieSecret    []byte
	CookieTTL       time.Duration
	Difficulty      int
	HoldSeconds     int
	BanDuration     time.Duration
	ChallengeTTL    time.Duration
	ChallengeAllNew bool
	TransparentTTL  time.Duration
}

// ProxyHandler is the main reverse proxy handler for Client_WAF.
type ProxyHandler struct {
	cookieMgr      *cookie.HMACCookieManager
	rateLimiter    *ratelimit.SlidingWindowLimiter
	banEngine      *ban.InMemoryBanEngine
	challengeStore *challenge.Store
	cookieMgrV2    *cookie.CookieManagerV2
	cookieLimiter  *ratelimit.CookieRateLimiter
	escalationEng  *escalation.EscalationEngine
	loopDetector   *challenge.LoopDetector
	cfExtractor    *cf.CFExtractor
	tlsExtractor   *fingerprint.TLSExtractor
	backendURL     string
	cookieSecret   []byte
	cookieTTL      time.Duration
	difficulty     int
	holdSeconds    int
	banDuration    time.Duration
	challengeTTL   time.Duration
	transparentTTL time.Duration
	challengeAll   bool
	reverseProxy   *httputil.ReverseProxy
}

// NewProxyHandler creates a new ProxyHandler with the given config and dependencies.
func NewProxyHandler(
	config ProxyConfig,
	cookieMgr *cookie.HMACCookieManager,
	rateLimiter *ratelimit.SlidingWindowLimiter,
	banEngine *ban.InMemoryBanEngine,
	challengeStore *challenge.Store,
) *ProxyHandler {
	if config.CookieTTL <= 0 {
		config.CookieTTL = 5 * time.Minute
	}
	if config.Difficulty <= 0 {
		config.Difficulty = challenge.DefaultDifficulty
	}
	if config.HoldSeconds <= 0 {
		config.HoldSeconds = challenge.DefaultHoldSeconds
	}
	if config.BanDuration <= 0 {
		config.BanDuration = 15 * time.Minute
	}
	if config.ChallengeTTL <= 0 {
		config.ChallengeTTL = 90 * time.Second
	}
	if config.TransparentTTL <= 0 {
		config.TransparentTTL = 30 * time.Second
	}

	backendTarget, _ := url.Parse(config.BackendURL)
	rp := httputil.NewSingleHostReverseProxy(backendTarget)

	// Optimized transport: connection pooling + keep-alive for high throughput
	rp.Transport = &http.Transport{
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 100,
		MaxConnsPerHost:     0, // unlimited
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true, // backend handles compression
		ForceAttemptHTTP2:   true, // HTTP/2 to backend if supported
	}

	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		ServeBranded502(w)
	}

	return &ProxyHandler{
		cookieMgr:      cookieMgr,
		rateLimiter:    rateLimiter,
		banEngine:      banEngine,
		challengeStore: challengeStore,
		backendURL:     config.BackendURL,
		cookieSecret:   config.CookieSecret,
		cookieTTL:      config.CookieTTL,
		difficulty:     config.Difficulty,
		holdSeconds:    config.HoldSeconds,
		banDuration:    config.BanDuration,
		challengeTTL:   config.ChallengeTTL,
		transparentTTL: config.TransparentTTL,
		challengeAll:   config.ChallengeAllNew,
		reverseProxy:   rp,
	}
}

// ServeHTTP implements http.Handler.
func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ip := h.extractClientIP(r)

	// Bypass WAF for internal kiro endpoints that must be publicly accessible
	// without challenge (install script, health check, API endpoints)
	if isPassthroughPath(r.URL.Path) {
		h.reverseProxy.ServeHTTP(w, r)
		return
	}

	if r.URL.Path == "/__kiro/challenge/verify" {
		h.handleChallengeVerify(w, r, ip)
		return
	}
	if r.URL.Path == "/__kiro/hold/verify" {
		h.handleHoldVerify(w, r, ip)
		return
	}
	if r.URL.Path == "/__kiro/transparent/verify" {
		h.handleTransparentVerify(w, r, ip)
		return
	}

	if ua.IsAutomationUA(r.UserAgent()) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if h.banEngine != nil && h.banEngine.IsBanned(ip) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// ALWAYS record request for rate limiting (volume tracking)
	if h.rateLimiter != nil {
		h.rateLimiter.RecordRequest(ip)
	}

	// Check if IP hit hard block threshold → ban immediately (even with valid cookie)
	if h.rateLimiter != nil && h.rateLimiter.IsHardBlocked(ip) {
		if h.banEngine != nil {
			h.banEngine.Ban(ip, h.banDuration, "rate_limit_hard_block")
		}
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Check if IP exceeded soft threshold → force challenge even with valid cookie
	// This is critical: prevents cookie holders from flooding indefinitely
	if h.rateLimiter != nil && !h.rateLimiter.Allow(ip) {
		// Invalidate cookie by not checking it — force re-challenge
		h.serveChallengeForLevel(w, r, ip)
		return
	}

	// Under rate limit — check cookie
	if cookieValue, valid := h.hasValidCookieV2(r, ip); valid {
		if h.cookieLimiter != nil && cookieValue != "" {
			if !h.cookieLimiter.RecordAndCheck(cookieValue) {
				h.serveChallengeForLevel(w, r, ip)
				return
			}
		}
		h.maybeRefreshCookie(w, r, ip)
		h.reverseProxy.ServeHTTP(w, r)
		return
	}

	// No valid cookie, under rate limit — serve challenge based on escalation level
	h.serveChallengeForLevel(w, r, ip)
}

func (h *ProxyHandler) serveChallengeForLevel(w http.ResponseWriter, r *http.Request, ip string) {
	level := h.getEscalationLevel(ip)

	// When rate limited (level >= 2 from volume), do NOT allow loop detector bypass
	// This prevents attackers from getting cookies by triggering loop detection
	rateLimited := h.rateLimiter != nil && !h.rateLimiter.Allow(ip)

	switch level {
	case 0:
		h.reverseProxy.ServeHTTP(w, r)
	case 1:
		if !rateLimited && h.shouldBypassLoop(ip, "transparent") {
			h.setAccessCookieV2(w, r, ip)
			h.reverseProxy.ServeHTTP(w, r)
			return
		}
		h.recordLoop(ip, "transparent")
		challenge.ServeTransparentPage(w, r, h.challengeStore, h.transparentTTL, ip)
	case 2:
		if !rateLimited && h.shouldBypassLoop(ip, "pow") {
			h.setAccessCookieV2(w, r, ip)
			h.reverseProxy.ServeHTTP(w, r)
			return
		}
		h.recordLoop(ip, "pow")
		challenge.ServeChallengePage(w, r, h.challengeStore, h.difficulty, h.challengeTTL, ip)
	case 3:
		if !rateLimited && h.shouldBypassLoop(ip, "hold") {
			h.setAccessCookieV2(w, r, ip)
			h.reverseProxy.ServeHTTP(w, r)
			return
		}
		h.recordLoop(ip, "hold")
		challenge.ServeHoldPage(w, r, h.challengeStore, h.holdSeconds, h.challengeTTL, ip)
	default:
		h.banEngine.Ban(ip, h.banDuration, "escalation_level_4")
		http.Error(w, "forbidden", http.StatusForbidden)
	}
}

func (h *ProxyHandler) handleTransparentVerify(w http.ResponseWriter, r *http.Request, ip string) {
	success := challenge.VerifyTransparent(w, r, h.challengeStore, ip, h.escalationEng)
	if success {
		// Reset rate limiter so user can browse after solving challenge
		if h.rateLimiter != nil {
			h.rateLimiter.ResetIP(ip)
		}
		h.setAccessCookieV2(w, r, ip)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
		if h.escalationEng != nil {
			h.escalationEng.RecordSuccess(ip)
		}
	}
}

func (h *ProxyHandler) handleChallengeVerify(w http.ResponseWriter, r *http.Request, ip string) {
	success := challenge.VerifyChallenge(w, r, h.challengeStore, ip)
	if success {
		if h.rateLimiter != nil {
			h.rateLimiter.ResetIP(ip)
		}
		h.setAccessCookieV2(w, r, ip)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
		if h.escalationEng != nil {
			h.escalationEng.RecordSuccess(ip)
		}
	}
}

func (h *ProxyHandler) handleHoldVerify(w http.ResponseWriter, r *http.Request, ip string) {
	success := challenge.VerifyHold(w, r, h.challengeStore, ip)
	if success {
		if h.rateLimiter != nil {
			h.rateLimiter.ResetIP(ip)
		}
		h.setAccessCookieV2(w, r, ip)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
		if h.escalationEng != nil {
			h.escalationEng.RecordSuccess(ip)
		}
	}
}

func (h *ProxyHandler) extractClientIP(r *http.Request) string {
	if h.cfExtractor != nil {
		return h.cfExtractor.ExtractClientIP(r)
	}
	return ClientIP(r)
}

func (h *ProxyHandler) hasValidCookieV2(r *http.Request, ip string) (string, bool) {
	c, err := r.Cookie("kiro_access")
	if err != nil {
		return "", false
	}
	if h.cookieLimiter != nil {
		if h.cookieLimiter.IsRevoked(c.Value) {
			return "", false
		}
	}
	if h.cookieMgrV2 != nil {
		tlsFP := h.extractTLSFingerprint(r)
		valid, _, _ := h.cookieMgrV2.ValidateCookie(c.Value, ip, tlsFP)
		if valid {
			return c.Value, true
		}
		return "", false
	}
	valid, _ := h.cookieMgr.ValidateCookie(c.Value, ip, h.cookieSecret)
	if valid {
		return c.Value, true
	}
	return "", false
}

func (h *ProxyHandler) maybeRefreshCookie(w http.ResponseWriter, r *http.Request, ip string) {
	if h.cookieMgrV2 == nil {
		return
	}
	c, err := r.Cookie("kiro_access")
	if err != nil {
		return
	}
	tlsFP := h.extractTLSFingerprint(r)
	valid, remainingTTL, _ := h.cookieMgrV2.ValidateCookie(c.Value, ip, tlsFP)
	if !valid {
		return
	}
	if h.cookieMgrV2.ShouldRefresh(remainingTTL, h.cookieTTL) {
		h.setAccessCookieV2(w, r, ip)
	}
}

func (h *ProxyHandler) setAccessCookieV2(w http.ResponseWriter, r *http.Request, ip string) {
	if h.cookieMgrV2 != nil {
		tlsFP := h.extractTLSFingerprint(r)
		cookieValue, err := h.cookieMgrV2.GenerateCookie(ip, tlsFP, h.cookieTTL)
		if err != nil {
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     "kiro_access",
			Value:    cookieValue,
			Path:     "/",
			MaxAge:   int(h.cookieTTL.Seconds()),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   true,
		})
		return
	}
	h.setAccessCookie(w, ip)
}

func (h *ProxyHandler) setAccessCookie(w http.ResponseWriter, ip string) {
	cookieValue, err := h.cookieMgr.GenerateCookie(ip, h.cookieSecret, h.cookieTTL)
	if err != nil {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "kiro_access",
		Value:    cookieValue,
		Path:     "/",
		MaxAge:   int(h.cookieTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
	})
}

func (h *ProxyHandler) extractTLSFingerprint(r *http.Request) string {
	if h.tlsExtractor != nil {
		return h.tlsExtractor.ExtractFingerprint(r)
	}
	return ""
}

func (h *ProxyHandler) getEscalationLevel(ip string) int {
	// Volume-based level from rate limiter
	rateLimitLevel := 0
	if h.rateLimiter != nil {
		if h.rateLimiter.IsHardBlocked(ip) {
			rateLimitLevel = 4
		} else if h.rateLimiter.IsDoubleThreshold(ip) {
			rateLimitLevel = 3 // Hold captcha for 2x soft threshold
		} else if !h.rateLimiter.Allow(ip) {
			rateLimitLevel = 2 // PoW challenge for soft threshold
		}
	}

	// Failure-based level from escalation engine
	escalationLevel := 0
	if h.escalationEng != nil {
		escalationLevel = h.escalationEng.GetLevel(ip)
		// If challenge_all_new is disabled, don't challenge new visitors (level 1)
		// Only challenge if escalation is due to actual failures (level >= 2)
		if !h.challengeAll && escalationLevel == 1 {
			escalationLevel = 0
		}
	} else if h.challengeAll {
		escalationLevel = 1
	}

	// Use the higher of the two levels
	if rateLimitLevel > escalationLevel {
		return rateLimitLevel
	}
	return escalationLevel
}

func (h *ProxyHandler) shouldBypassLoop(ip string, challengeType string) bool {
	if h.loopDetector != nil {
		return h.loopDetector.ShouldBypass(ip, challengeType)
	}
	return false
}

func (h *ProxyHandler) recordLoop(ip string, challengeType string) {
	if h.loopDetector != nil {
		h.loopDetector.Record(ip, challengeType)
	}
}

func (h *ProxyHandler) hasValidCookie(r *http.Request, ip string) bool {
	c, err := r.Cookie("kiro_access")
	if err != nil {
		return false
	}
	valid, _ := h.cookieMgr.ValidateCookie(c.Value, ip, h.cookieSecret)
	return valid
}

// isPassthroughPath returns true for paths that should bypass WAF challenge
// and be proxied directly to the backend.
// Only Kiro internal endpoints bypass — backend APIs are still protected by WAF.
func isPassthroughPath(path string) bool {
	switch {
	case path == "/install" || path == "/install.sh":
		return true
	case path == "/healthz":
		return true
	// Only Kiro master API endpoints bypass (heartbeat, download, register)
	case len(path) >= 12 && path[:12] == "/api/v1/hear":
		return true
	case len(path) >= 16 && path[:16] == "/api/v1/download":
		return true
	case len(path) >= 16 && path[:16] == "/api/v1/register":
		return true
	case len(path) >= 14 && path[:14] == "/api/v1/update":
		return true
	case len(path) >= 6 && path[:6] == "/docs/":
		return true
	case path == "/docs":
		return true
	case len(path) >= 7 && path[:7] == "/admin/":
		return true
	case path == "/admin":
		return true
	case len(path) >= 8 && path[:8] == "/static/":
		return true
	}
	return false
}
