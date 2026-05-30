// Package cookie triển khai HMAC-SHA256 access cookie cho Client_WAF.
// Cookie được binding với IP address và có expiration timestamp.
package cookie

import "time"

// CookieManager định nghĩa interface cho HMAC cookie operations.
// Implementations phải đảm bảo cookie binding với IP và kiểm tra expiration.
type CookieManager interface {
	// GenerateCookie tạo một access cookie mới cho IP với secret và TTL cho trước.
	// Cookie chứa HMAC-SHA256 signature binding với IP và expiration timestamp.
	GenerateCookie(ip string, secret []byte, ttl time.Duration) (string, error)

	// ValidateCookie xác thực cookie: kiểm tra HMAC signature, IP match, và expiration.
	// Trả về true nếu cookie hợp lệ, false nếu không hợp lệ hoặc đã hết hạn.
	ValidateCookie(cookie string, ip string, secret []byte) (bool, error)
}
