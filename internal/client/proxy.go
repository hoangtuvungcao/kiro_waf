// Package client triển khai reverse proxy handler chính cho Client_WAF.
// ProxyHandler tích hợp tất cả components: cookie, ratelimit, ban, ua, challenge.
// Luồng xử lý: check UA → check ban → check cookie → rate limit → challenge/ban/proxy.
package client

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"kiro_waf/internal/client/ban"
	"kiro_waf/internal/client/challenge"
	"kiro_waf/internal/client/cookie"
	"kiro_waf/internal/client/ratelimit"
	"kiro_waf/internal/client/ua"
)

// ProxyConfig chứa cấu hình cho ProxyHandler.
type ProxyConfig struct {
	// BackendURL là URL của backend server để proxy verified requests.
	BackendURL string

	// CookieSecret là HMAC-SHA256 secret key cho access cookie.
	CookieSecret []byte

	// CookieTTL là thời gian sống của access cookie.
	CookieTTL time.Duration

	// Difficulty là số ký tự "0" prefix cho PoW challenge.
	Difficulty int

	// HoldSeconds là thời gian giữ tối thiểu cho Hold captcha.
	HoldSeconds int

	// BanDuration là thời gian ban khi IP vượt hard threshold.
	BanDuration time.Duration

	// ChallengeTTL là thời gian sống của challenge token.
	ChallengeTTL time.Duration

	// ChallengeAllNew khi true, mọi request không có cookie hợp lệ sẽ bị challenge ngay.
	ChallengeAllNew bool
}

// ProxyHandler là reverse proxy handler chính cho Client_WAF.
// Tích hợp tất cả components: cookie, ratelimit, ban, ua, challenge.
type ProxyHandler struct {
	cookieMgr      *cookie.HMACCookieManager
	rateLimiter    *ratelimit.SlidingWindowLimiter
	banEngine      *ban.InMemoryBanEngine
	challengeStore *challenge.Store
	backendURL     string
	cookieSecret   []byte
	cookieTTL      time.Duration
	difficulty     int
	holdSeconds    int
	banDuration    time.Duration
	challengeTTL   time.Duration
	challengeAll   bool
	reverseProxy   *httputil.ReverseProxy
}

// NewProxyHandler tạo ProxyHandler mới với cấu hình và dependencies cho trước.
func NewProxyHandler(
	config ProxyConfig,
	cookieMgr *cookie.HMACCookieManager,
	rateLimiter *ratelimit.SlidingWindowLimiter,
	banEngine *ban.InMemoryBanEngine,
	challengeStore *challenge.Store,
) *ProxyHandler {
	// Set defaults
	if config.CookieTTL <= 0 {
		config.CookieTTL = 20 * time.Minute
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

	backendTarget, _ := url.Parse(config.BackendURL)
	rp := httputil.NewSingleHostReverseProxy(backendTarget)
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
		challengeAll:   config.ChallengeAllNew,
		reverseProxy:   rp,
	}
}

// ServeHTTP implements http.Handler.
// Luồng xử lý:
//  1. Check User-Agent → if automation tool, block immediately (403)
//  2. Check if IP is banned → if yes, return 403
//  3. Check access cookie (HMAC-SHA256, IP-bound) → if valid, proxy to backend
//  4. If no valid cookie:
//     a. Record request in rate limiter
//     b. If per-IP exceeds hard threshold → ban IP + return 403
//     c. If per-IP exceeds soft threshold → serve Hold captcha
//     d. If per-subnet exceeds threshold → serve PoW challenge
//     e. Otherwise → proxy to backend
func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle challenge verify endpoints
	if r.URL.Path == "/__kiro/challenge/verify" {
		h.handleChallengeVerify(w, r)
		return
	}
	if r.URL.Path == "/__kiro/hold/verify" {
		h.handleHoldVerify(w, r)
		return
	}

	ip := ClientIP(r)

	// Step 1: Check User-Agent for automation tools
	if ua.IsAutomationUA(r.UserAgent()) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Step 2: Check if IP is banned
	if h.banEngine.IsBanned(ip) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Step 3: Check access cookie
	if h.hasValidCookie(r, ip) {
		h.reverseProxy.ServeHTTP(w, r)
		return
	}

	// Step 3b: If challenge-all-new mode, serve PoW challenge immediately
	if h.challengeAll {
		challenge.ServeChallengePage(w, r, h.challengeStore, h.difficulty, h.challengeTTL, ip)
		return
	}

	// Step 4: No valid cookie — apply rate limiting
	h.rateLimiter.RecordRequest(ip)

	// Step 4b: Check hard threshold → ban
	if h.rateLimiter.IsHardBlocked(ip) {
		h.banEngine.Ban(ip, h.banDuration, "hard_threshold_exceeded")
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Step 4c: Check soft threshold (per-IP) → Hold captcha
	if !h.rateLimiter.Allow(ip) {
		challenge.ServeHoldPage(w, r, h.challengeStore, h.holdSeconds, h.challengeTTL, ip)
		return
	}

	// Step 4d: Check subnet threshold → PoW challenge
	subnet := h.rateLimiter.GetSubnet24(ip)
	if !h.rateLimiter.AllowSubnet(subnet) {
		challenge.ServeChallengePage(w, r, h.challengeStore, h.difficulty, h.challengeTTL, ip)
		return
	}

	// Step 4e: Under all thresholds → proxy to backend
	h.reverseProxy.ServeHTTP(w, r)
}

// handleChallengeVerify xử lý POST /__kiro/challenge/verify.
// Khi PoW solution hợp lệ, set access cookie và trả về 200.
func (h *ProxyHandler) handleChallengeVerify(w http.ResponseWriter, r *http.Request) {
	ip := ClientIP(r)
	h.setAccessCookie(w, ip)
	challenge.VerifyChallenge(w, r, h.challengeStore, ip)
}

// handleHoldVerify xử lý POST /__kiro/hold/verify.
// Khi hold duration đủ, set access cookie và trả về 200.
func (h *ProxyHandler) handleHoldVerify(w http.ResponseWriter, r *http.Request) {
	ip := ClientIP(r)
	// Set cookie BEFORE VerifyHold writes the response body,
	// because headers must be set before WriteHeader is called.
	// We pre-set the cookie; if verification fails, the cookie won't matter
	// because the response will be 4xx and browser won't redirect.
	h.setAccessCookie(w, ip)
	challenge.VerifyHold(w, r, h.challengeStore, ip)
}

// hasValidCookie kiểm tra request có access cookie hợp lệ hay không.
func (h *ProxyHandler) hasValidCookie(r *http.Request, ip string) bool {
	c, err := r.Cookie("kiro_access")
	if err != nil {
		return false
	}
	valid, _ := h.cookieMgr.ValidateCookie(c.Value, ip, h.cookieSecret)
	return valid
}

// setAccessCookie tạo và set access cookie cho client.
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
