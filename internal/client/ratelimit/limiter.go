// Package ratelimit triển khai rate limiting per-IP và per-subnet cho Client_WAF.
// SlidingWindowLimiter sử dụng sliding window algorithm với ngưỡng soft (challenge) và hard (ban).
package ratelimit

import (
	"net"
	"sync"
	"time"
)

// LimiterConfig chứa cấu hình cho SlidingWindowLimiter.
type LimiterConfig struct {
	// SoftThreshold: số request per-IP trong window trước khi trigger challenge.
	SoftThreshold int
	// HardThreshold: số request per-IP trong window trước khi trigger ban.
	HardThreshold int
	// SubnetThreshold: số request per-subnet /24 trong window trước khi trigger challenge/ban.
	SubnetThreshold int
	// WindowDuration: kích thước sliding window (ví dụ: 60 giây).
	WindowDuration time.Duration
}

// requestEntry lưu timestamp của một request trong sliding window.
type requestEntry struct {
	timestamp time.Time
}

// ipState lưu trạng thái rate limiting cho một IP.
type ipState struct {
	requests []requestEntry
}

// subnetState lưu trạng thái rate limiting cho một subnet /24.
type subnetState struct {
	requests []requestEntry
}

// SlidingWindowLimiter triển khai RateLimiter interface với sliding window algorithm.
// Thread-safe thông qua sync.Mutex.
type SlidingWindowLimiter struct {
	mu      sync.Mutex
	config  LimiterConfig
	ipMap   map[string]*ipState
	subMap  map[string]*subnetState
	nowFunc func() time.Time // cho phép inject thời gian trong tests
}

// NewSlidingWindowLimiter tạo một SlidingWindowLimiter mới với cấu hình cho trước.
func NewSlidingWindowLimiter(config LimiterConfig) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		config:  config,
		ipMap:   make(map[string]*ipState),
		subMap:  make(map[string]*subnetState),
		nowFunc: time.Now,
	}
}

// Allow kiểm tra xem IP có được phép gửi request hay không.
// Trả về true nếu IP chưa vượt ngưỡng soft threshold (per-IP).
func (s *SlidingWindowLimiter) Allow(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.nowFunc()
	state := s.getOrCreateIPState(ip)
	s.pruneIPState(state, now)

	return len(state.requests) < s.config.SoftThreshold
}

// AllowSubnet kiểm tra xem subnet /24 có được phép hay không.
// Trả về true nếu subnet chưa vượt ngưỡng subnet threshold.
func (s *SlidingWindowLimiter) AllowSubnet(subnet string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.nowFunc()
	state := s.getOrCreateSubnetState(subnet)
	s.pruneSubnetState(state, now)

	return len(state.requests) < s.config.SubnetThreshold
}

// RecordRequest ghi nhận một request từ IP.
// Cập nhật bộ đếm cho cả per-IP và per-subnet /24 tracking.
func (s *SlidingWindowLimiter) RecordRequest(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.nowFunc()
	entry := requestEntry{timestamp: now}

	// Record per-IP
	ipSt := s.getOrCreateIPState(ip)
	s.pruneIPState(ipSt, now)
	ipSt.requests = append(ipSt.requests, entry)

	// Record per-subnet /24
	subnet := getSubnet24(ip)
	subSt := s.getOrCreateSubnetState(subnet)
	s.pruneSubnetState(subSt, now)
	subSt.requests = append(subSt.requests, entry)
}

// IsHardBlocked kiểm tra xem IP có vượt ngưỡng hard threshold hay không.
// Trả về true nếu số request từ IP trong window >= HardThreshold.
func (s *SlidingWindowLimiter) IsHardBlocked(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.nowFunc()
	state, exists := s.ipMap[ip]
	if !exists {
		return false
	}
	s.pruneIPState(state, now)

	return len(state.requests) >= s.config.HardThreshold
}

// GetSubnet24 trả về subnet /24 string cho một IP address.
// Ví dụ: "192.168.1.100" → "192.168.1.0/24"
func (s *SlidingWindowLimiter) GetSubnet24(ip string) string {
	return getSubnet24(ip)
}

// Cleanup xóa các entries đã hết hạn khỏi tất cả IP và subnet maps.
// Nên được gọi định kỳ để giải phóng bộ nhớ.
func (s *SlidingWindowLimiter) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.nowFunc()

	// Cleanup IP entries
	for ip, state := range s.ipMap {
		s.pruneIPState(state, now)
		if len(state.requests) == 0 {
			delete(s.ipMap, ip)
		}
	}

	// Cleanup subnet entries
	for subnet, state := range s.subMap {
		s.pruneSubnetState(state, now)
		if len(state.requests) == 0 {
			delete(s.subMap, subnet)
		}
	}
}

// SetNowFunc sets the time function used by the limiter (for testing).
func (s *SlidingWindowLimiter) SetNowFunc(fn func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nowFunc = fn
}

// HasIP returns true if the IP has any entries in the limiter's IP map.
func (s *SlidingWindowLimiter) HasIP(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.ipMap[ip]
	return exists
}

// HasSubnet returns true if the subnet has any entries in the limiter's subnet map.
func (s *SlidingWindowLimiter) HasSubnet(subnet string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.subMap[subnet]
	return exists
}

// getOrCreateIPState lấy hoặc tạo mới ipState cho IP.
// Caller phải giữ lock.
func (s *SlidingWindowLimiter) getOrCreateIPState(ip string) *ipState {
	state, exists := s.ipMap[ip]
	if !exists {
		state = &ipState{}
		s.ipMap[ip] = state
	}
	return state
}

// getOrCreateSubnetState lấy hoặc tạo mới subnetState cho subnet.
// Caller phải giữ lock.
func (s *SlidingWindowLimiter) getOrCreateSubnetState(subnet string) *subnetState {
	state, exists := s.subMap[subnet]
	if !exists {
		state = &subnetState{}
		s.subMap[subnet] = state
	}
	return state
}

// pruneIPState loại bỏ các request entries nằm ngoài sliding window.
// Caller phải giữ lock.
func (s *SlidingWindowLimiter) pruneIPState(state *ipState, now time.Time) {
	cutoff := now.Add(-s.config.WindowDuration)
	i := 0
	for i < len(state.requests) && state.requests[i].timestamp.Before(cutoff) {
		i++
	}
	if i > 0 {
		state.requests = state.requests[i:]
	}
}

// pruneSubnetState loại bỏ các request entries nằm ngoài sliding window.
// Caller phải giữ lock.
func (s *SlidingWindowLimiter) pruneSubnetState(state *subnetState, now time.Time) {
	cutoff := now.Add(-s.config.WindowDuration)
	i := 0
	for i < len(state.requests) && state.requests[i].timestamp.Before(cutoff) {
		i++
	}
	if i > 0 {
		state.requests = state.requests[i:]
	}
}

// getSubnet24 trích xuất subnet /24 từ một IP address string.
// Trả về dạng "x.x.x.0/24". Nếu IP không hợp lệ, trả về "0.0.0.0/24".
func getSubnet24(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "0.0.0.0/24"
	}
	// Đảm bảo sử dụng IPv4
	ipv4 := parsed.To4()
	if ipv4 == nil {
		return "0.0.0.0/24"
	}
	// Mask với /24
	mask := net.CIDRMask(24, 32)
	subnet := ipv4.Mask(mask)
	return subnet.String() + "/24"
}
