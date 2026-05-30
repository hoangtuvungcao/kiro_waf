// Package ratelimit triển khai rate limiting per-IP và per-subnet cho Client_WAF.
// CookieRateLimiter triển khai per-cookie rate limiting với O(1) lookup sử dụng FNV-1a hash.
package ratelimit

import (
	"hash/fnv"
	"sync"
	"time"
)

// CookieRateLimiterConfig chứa cấu hình cho CookieRateLimiter.
type CookieRateLimiterConfig struct {
	// Threshold: số request tối đa per-cookie trong window trước khi revoke.
	Threshold int
	// Window: kích thước time window (ví dụ: 60 giây).
	Window time.Duration
}

// cookieCounter lưu trạng thái rate limiting cho một cookie.
type cookieCounter struct {
	count       int
	windowStart time.Time
}

// CookieRateLimiter triển khai per-cookie rate limiting với O(1) lookup.
// Sử dụng FNV-1a hash của cookie value làm map key.
// Thread-safe thông qua sync.Mutex.
type CookieRateLimiter struct {
	mu        sync.Mutex
	counters  map[uint64]*cookieCounter // FNV-1a hash of cookie → counter
	revoked   map[uint64]time.Time      // revoked cookie hashes → revocation time
	threshold int                       // max requests per window
	window    time.Duration             // time window duration
	nowFunc   func() time.Time          // cho phép inject thời gian trong tests
}

// NewCookieRateLimiter tạo một CookieRateLimiter mới với cấu hình cho trước.
func NewCookieRateLimiter(config CookieRateLimiterConfig) *CookieRateLimiter {
	return &CookieRateLimiter{
		counters:  make(map[uint64]*cookieCounter),
		revoked:   make(map[uint64]time.Time),
		threshold: config.Threshold,
		window:    config.Window,
		nowFunc:   time.Now,
	}
}

// RecordAndCheck ghi nhận một request cho cookie và kiểm tra rate limit.
// Trả về true nếu cookie vẫn hợp lệ (chưa bị revoke và chưa vượt threshold).
// Trả về false nếu cookie đã bị revoke hoặc vượt threshold (cookie sẽ bị revoke).
func (c *CookieRateLimiter) RecordAndCheck(cookieValue string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	hash := hashCookie(cookieValue)
	now := c.nowFunc()

	// Kiểm tra xem cookie đã bị revoke chưa
	if _, revoked := c.revoked[hash]; revoked {
		return false
	}

	// Lấy hoặc tạo counter cho cookie
	counter, exists := c.counters[hash]
	if !exists {
		counter = &cookieCounter{
			count:       0,
			windowStart: now,
		}
		c.counters[hash] = counter
	}

	// Reset counter nếu window đã hết hạn (fixed window with reset)
	if now.Sub(counter.windowStart) >= c.window {
		counter.count = 0
		counter.windowStart = now
	}

	// Increment counter
	counter.count++

	// Kiểm tra threshold
	if counter.count > c.threshold {
		// Revoke cookie
		c.revoked[hash] = now
		delete(c.counters, hash)
		return false
	}

	return true
}

// IsRevoked kiểm tra xem cookie có bị revoke hay không.
// Trả về true nếu cookie đã bị revoke.
func (c *CookieRateLimiter) IsRevoked(cookieValue string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	hash := hashCookie(cookieValue)
	_, revoked := c.revoked[hash]
	return revoked
}

// Cleanup xóa các counter đã hết hạn và các revocation entries cũ.
// Nên được gọi định kỳ để giải phóng bộ nhớ và ngăn unbounded memory growth.
func (c *CookieRateLimiter) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.nowFunc()

	// Xóa counters đã hết hạn (window đã qua)
	for hash, counter := range c.counters {
		if now.Sub(counter.windowStart) >= c.window {
			delete(c.counters, hash)
		}
	}

	// Xóa revocation entries cũ hơn 2× window (cho phép cleanup nhưng giữ revocation đủ lâu)
	revocationExpiry := 2 * c.window
	for hash, revokedAt := range c.revoked {
		if now.Sub(revokedAt) >= revocationExpiry {
			delete(c.revoked, hash)
		}
	}
}

// SetNowFunc sets the time function used by the limiter (for testing).
func (c *CookieRateLimiter) SetNowFunc(fn func() time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nowFunc = fn
}

// hashCookie tính FNV-1a hash của cookie value cho O(1) map lookup.
func hashCookie(cookieValue string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(cookieValue))
	return h.Sum64()
}
