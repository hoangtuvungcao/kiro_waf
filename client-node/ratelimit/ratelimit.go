// Package ratelimit triển khai rate limiting per-IP và per-subnet cho Client_WAF.
// Sử dụng sliding window algorithm với ngưỡng soft (challenge) và hard (ban).
package ratelimit

// RateLimiter định nghĩa interface cho rate limiting operations.
// Implementations phải hỗ trợ per-IP và per-subnet /24 rate limiting độc lập.
type RateLimiter interface {
	// Allow kiểm tra xem IP có được phép gửi request hay không.
	// Trả về true nếu IP chưa vượt ngưỡng rate limit.
	Allow(ip string) bool

	// AllowSubnet kiểm tra xem subnet /24 có được phép hay không.
	// Trả về true nếu subnet chưa vượt ngưỡng rate limit.
	AllowSubnet(subnet string) bool

	// RecordRequest ghi nhận một request từ IP.
	// Cập nhật bộ đếm cho cả per-IP và per-subnet tracking.
	RecordRequest(ip string)
}
