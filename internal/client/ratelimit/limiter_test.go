package ratelimit

import (
	"sync"
	"testing"
	"time"
)

func TestNewSlidingWindowLimiter(t *testing.T) {
	config := LimiterConfig{
		SoftThreshold:   10,
		HardThreshold:   20,
		SubnetThreshold: 50,
		WindowDuration:  60 * time.Second,
	}
	limiter := NewSlidingWindowLimiter(config)

	if limiter == nil {
		t.Fatal("NewSlidingWindowLimiter returned nil")
	}
	if limiter.config.SoftThreshold != 10 {
		t.Errorf("expected SoftThreshold=10, got %d", limiter.config.SoftThreshold)
	}
	if limiter.config.HardThreshold != 20 {
		t.Errorf("expected HardThreshold=20, got %d", limiter.config.HardThreshold)
	}
	if limiter.config.SubnetThreshold != 50 {
		t.Errorf("expected SubnetThreshold=50, got %d", limiter.config.SubnetThreshold)
	}
}

func TestAllow_UnderThreshold(t *testing.T) {
	config := LimiterConfig{
		SoftThreshold:   5,
		HardThreshold:   10,
		SubnetThreshold: 20,
		WindowDuration:  60 * time.Second,
	}
	limiter := NewSlidingWindowLimiter(config)

	ip := "192.168.1.100"

	// Record 4 requests (under soft threshold of 5)
	for i := 0; i < 4; i++ {
		limiter.RecordRequest(ip)
	}

	if !limiter.Allow(ip) {
		t.Error("expected Allow=true when under soft threshold")
	}
}

func TestAllow_AtSoftThreshold(t *testing.T) {
	config := LimiterConfig{
		SoftThreshold:   5,
		HardThreshold:   10,
		SubnetThreshold: 20,
		WindowDuration:  60 * time.Second,
	}
	limiter := NewSlidingWindowLimiter(config)

	ip := "192.168.1.100"

	// Record exactly 5 requests (at soft threshold)
	for i := 0; i < 5; i++ {
		limiter.RecordRequest(ip)
	}

	if limiter.Allow(ip) {
		t.Error("expected Allow=false when at soft threshold")
	}
}

func TestAllow_OverSoftThreshold(t *testing.T) {
	config := LimiterConfig{
		SoftThreshold:   5,
		HardThreshold:   10,
		SubnetThreshold: 20,
		WindowDuration:  60 * time.Second,
	}
	limiter := NewSlidingWindowLimiter(config)

	ip := "192.168.1.100"

	// Record 7 requests (over soft threshold)
	for i := 0; i < 7; i++ {
		limiter.RecordRequest(ip)
	}

	if limiter.Allow(ip) {
		t.Error("expected Allow=false when over soft threshold")
	}
}

func TestIsHardBlocked(t *testing.T) {
	config := LimiterConfig{
		SoftThreshold:   5,
		HardThreshold:   10,
		SubnetThreshold: 20,
		WindowDuration:  60 * time.Second,
	}
	limiter := NewSlidingWindowLimiter(config)

	ip := "10.0.0.1"

	// Under hard threshold
	for i := 0; i < 9; i++ {
		limiter.RecordRequest(ip)
	}
	if limiter.IsHardBlocked(ip) {
		t.Error("expected IsHardBlocked=false when under hard threshold")
	}

	// At hard threshold
	limiter.RecordRequest(ip)
	if !limiter.IsHardBlocked(ip) {
		t.Error("expected IsHardBlocked=true when at hard threshold")
	}
}

func TestIsHardBlocked_UnknownIP(t *testing.T) {
	config := LimiterConfig{
		SoftThreshold:   5,
		HardThreshold:   10,
		SubnetThreshold: 20,
		WindowDuration:  60 * time.Second,
	}
	limiter := NewSlidingWindowLimiter(config)

	if limiter.IsHardBlocked("1.2.3.4") {
		t.Error("expected IsHardBlocked=false for unknown IP")
	}
}

func TestAllowSubnet_UnderThreshold(t *testing.T) {
	config := LimiterConfig{
		SoftThreshold:   100,
		HardThreshold:   200,
		SubnetThreshold: 10,
		WindowDuration:  60 * time.Second,
	}
	limiter := NewSlidingWindowLimiter(config)

	// Record requests from different IPs in same /24 subnet
	for i := 0; i < 9; i++ {
		limiter.RecordRequest("192.168.1." + string(rune('1'+i)))
	}

	subnet := limiter.GetSubnet24("192.168.1.1")
	if !limiter.AllowSubnet(subnet) {
		t.Error("expected AllowSubnet=true when under subnet threshold")
	}
}

func TestAllowSubnet_AtThreshold(t *testing.T) {
	config := LimiterConfig{
		SoftThreshold:   100,
		HardThreshold:   200,
		SubnetThreshold: 10,
		WindowDuration:  60 * time.Second,
	}
	limiter := NewSlidingWindowLimiter(config)

	// Record 10 requests from IPs in same /24 subnet
	for i := 1; i <= 10; i++ {
		ip := "192.168.1." + itoa(i)
		limiter.RecordRequest(ip)
	}

	subnet := limiter.GetSubnet24("192.168.1.1")
	if limiter.AllowSubnet(subnet) {
		t.Error("expected AllowSubnet=false when at subnet threshold")
	}
}

func TestSubnetIndependence(t *testing.T) {
	config := LimiterConfig{
		SoftThreshold:   100,
		HardThreshold:   200,
		SubnetThreshold: 5,
		WindowDuration:  60 * time.Second,
	}
	limiter := NewSlidingWindowLimiter(config)

	// Flood subnet 10.0.1.0/24
	for i := 1; i <= 10; i++ {
		ip := "10.0.1." + itoa(i)
		limiter.RecordRequest(ip)
	}

	// Subnet 10.0.1.0/24 should be blocked
	subnet1 := limiter.GetSubnet24("10.0.1.1")
	if limiter.AllowSubnet(subnet1) {
		t.Error("expected subnet 10.0.1.0/24 to be blocked")
	}

	// Subnet 10.0.2.0/24 should still be allowed
	subnet2 := limiter.GetSubnet24("10.0.2.1")
	if !limiter.AllowSubnet(subnet2) {
		t.Error("expected subnet 10.0.2.0/24 to still be allowed")
	}
}

func TestSlidingWindowExpiry(t *testing.T) {
	config := LimiterConfig{
		SoftThreshold:   5,
		HardThreshold:   10,
		SubnetThreshold: 20,
		WindowDuration:  60 * time.Second,
	}
	limiter := NewSlidingWindowLimiter(config)

	now := time.Now()
	limiter.nowFunc = func() time.Time { return now }

	ip := "172.16.0.1"

	// Record 5 requests at time=now (at soft threshold)
	for i := 0; i < 5; i++ {
		limiter.RecordRequest(ip)
	}

	if limiter.Allow(ip) {
		t.Error("expected Allow=false at soft threshold")
	}

	// Advance time past window duration
	limiter.nowFunc = func() time.Time { return now.Add(61 * time.Second) }

	// Now the old requests should be expired
	if !limiter.Allow(ip) {
		t.Error("expected Allow=true after window expiry")
	}
}

func TestSlidingWindowPartialExpiry(t *testing.T) {
	config := LimiterConfig{
		SoftThreshold:   5,
		HardThreshold:   10,
		SubnetThreshold: 20,
		WindowDuration:  60 * time.Second,
	}
	limiter := NewSlidingWindowLimiter(config)

	now := time.Now()
	limiter.nowFunc = func() time.Time { return now }

	ip := "172.16.0.1"

	// Record 3 requests at t=0
	for i := 0; i < 3; i++ {
		limiter.RecordRequest(ip)
	}

	// Advance 30 seconds, record 3 more
	limiter.nowFunc = func() time.Time { return now.Add(30 * time.Second) }
	for i := 0; i < 3; i++ {
		limiter.RecordRequest(ip)
	}

	// At t=30s, total = 6 (over soft threshold)
	if limiter.Allow(ip) {
		t.Error("expected Allow=false with 6 requests in window")
	}

	// Advance to t=61s: first 3 requests expire, only 3 remain
	limiter.nowFunc = func() time.Time { return now.Add(61 * time.Second) }
	if !limiter.Allow(ip) {
		t.Error("expected Allow=true after partial expiry (3 requests remain)")
	}
}

func TestGetSubnet24(t *testing.T) {
	limiter := NewSlidingWindowLimiter(LimiterConfig{})

	tests := []struct {
		ip       string
		expected string
	}{
		{"192.168.1.100", "192.168.1.0/24"},
		{"192.168.1.1", "192.168.1.0/24"},
		{"192.168.1.255", "192.168.1.0/24"},
		{"10.0.0.1", "10.0.0.0/24"},
		{"172.16.5.200", "172.16.5.0/24"},
		{"8.8.8.8", "8.8.8.0/24"},
		{"invalid-ip", "0.0.0.0/24"},
		{"", "0.0.0.0/24"},
	}

	for _, tt := range tests {
		result := limiter.GetSubnet24(tt.ip)
		if result != tt.expected {
			t.Errorf("GetSubnet24(%q) = %q, want %q", tt.ip, result, tt.expected)
		}
	}
}

func TestGetSubnet24_Function(t *testing.T) {
	tests := []struct {
		ip       string
		expected string
	}{
		{"192.168.1.100", "192.168.1.0/24"},
		{"10.255.255.255", "10.255.255.0/24"},
		{"0.0.0.0", "0.0.0.0/24"},
		{"255.255.255.255", "255.255.255.0/24"},
		{"not-an-ip", "0.0.0.0/24"},
	}

	for _, tt := range tests {
		result := getSubnet24(tt.ip)
		if result != tt.expected {
			t.Errorf("getSubnet24(%q) = %q, want %q", tt.ip, result, tt.expected)
		}
	}
}

func TestCleanup(t *testing.T) {
	config := LimiterConfig{
		SoftThreshold:   5,
		HardThreshold:   10,
		SubnetThreshold: 20,
		WindowDuration:  60 * time.Second,
	}
	limiter := NewSlidingWindowLimiter(config)

	now := time.Now()
	limiter.nowFunc = func() time.Time { return now }

	// Record requests from multiple IPs
	limiter.RecordRequest("1.1.1.1")
	limiter.RecordRequest("2.2.2.2")
	limiter.RecordRequest("3.3.3.3")

	// Advance past window
	limiter.nowFunc = func() time.Time { return now.Add(61 * time.Second) }

	limiter.Cleanup()

	// All entries should be cleaned up
	limiter.mu.Lock()
	ipCount := len(limiter.ipMap)
	subCount := len(limiter.subMap)
	limiter.mu.Unlock()

	if ipCount != 0 {
		t.Errorf("expected 0 IP entries after cleanup, got %d", ipCount)
	}
	if subCount != 0 {
		t.Errorf("expected 0 subnet entries after cleanup, got %d", subCount)
	}
}

func TestCleanup_PartialExpiry(t *testing.T) {
	config := LimiterConfig{
		SoftThreshold:   5,
		HardThreshold:   10,
		SubnetThreshold: 20,
		WindowDuration:  60 * time.Second,
	}
	limiter := NewSlidingWindowLimiter(config)

	now := time.Now()
	limiter.nowFunc = func() time.Time { return now }

	// Record old request
	limiter.RecordRequest("1.1.1.1")

	// Record recent request from different IP
	limiter.nowFunc = func() time.Time { return now.Add(50 * time.Second) }
	limiter.RecordRequest("2.2.2.2")

	// Advance to t=61s: 1.1.1.1 expired, 2.2.2.2 still valid
	limiter.nowFunc = func() time.Time { return now.Add(61 * time.Second) }
	limiter.Cleanup()

	limiter.mu.Lock()
	_, has1 := limiter.ipMap["1.1.1.1"]
	_, has2 := limiter.ipMap["2.2.2.2"]
	limiter.mu.Unlock()

	if has1 {
		t.Error("expected 1.1.1.1 to be cleaned up")
	}
	if !has2 {
		t.Error("expected 2.2.2.2 to still exist")
	}
}

func TestConcurrentAccess(t *testing.T) {
	config := LimiterConfig{
		SoftThreshold:   1000,
		HardThreshold:   2000,
		SubnetThreshold: 5000,
		WindowDuration:  60 * time.Second,
	}
	limiter := NewSlidingWindowLimiter(config)

	var wg sync.WaitGroup
	numGoroutines := 10
	requestsPerGoroutine := 100

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ip := "10.0.0." + itoa(id+1)
			for i := 0; i < requestsPerGoroutine; i++ {
				limiter.RecordRequest(ip)
				limiter.Allow(ip)
				limiter.AllowSubnet(limiter.GetSubnet24(ip))
				limiter.IsHardBlocked(ip)
			}
		}(g)
	}

	wg.Wait()

	// Should not panic or deadlock - if we reach here, concurrency is safe
}

func TestPerIPAndSubnetIndependentTracking(t *testing.T) {
	config := LimiterConfig{
		SoftThreshold:   100, // High per-IP threshold
		HardThreshold:   200,
		SubnetThreshold: 5, // Low subnet threshold
		WindowDuration:  60 * time.Second,
	}
	limiter := NewSlidingWindowLimiter(config)

	// Each individual IP is under per-IP threshold
	// But combined subnet traffic exceeds subnet threshold
	for i := 1; i <= 6; i++ {
		ip := "192.168.1." + itoa(i)
		limiter.RecordRequest(ip)
	}

	// Each IP should still be allowed (only 1 request each, threshold=100)
	if !limiter.Allow("192.168.1.1") {
		t.Error("individual IP should be allowed (1 request, threshold=100)")
	}

	// But subnet should be blocked (6 requests, threshold=5)
	subnet := limiter.GetSubnet24("192.168.1.1")
	if limiter.AllowSubnet(subnet) {
		t.Error("subnet should be blocked (6 requests, threshold=5)")
	}
}

func TestRecordRequest_UpdatesBothIPAndSubnet(t *testing.T) {
	config := LimiterConfig{
		SoftThreshold:   10,
		HardThreshold:   20,
		SubnetThreshold: 30,
		WindowDuration:  60 * time.Second,
	}
	limiter := NewSlidingWindowLimiter(config)

	ip := "10.20.30.40"
	limiter.RecordRequest(ip)

	limiter.mu.Lock()
	ipSt := limiter.ipMap[ip]
	subSt := limiter.subMap["10.20.30.0/24"]
	limiter.mu.Unlock()

	if ipSt == nil || len(ipSt.requests) != 1 {
		t.Error("expected 1 request in IP state")
	}
	if subSt == nil || len(subSt.requests) != 1 {
		t.Error("expected 1 request in subnet state")
	}
}

func TestImplementsRateLimiterInterface(t *testing.T) {
	config := LimiterConfig{
		SoftThreshold:   10,
		HardThreshold:   20,
		SubnetThreshold: 50,
		WindowDuration:  60 * time.Second,
	}
	limiter := NewSlidingWindowLimiter(config)

	// Verify that SlidingWindowLimiter implements RateLimiter interface
	var _ RateLimiter = limiter
}

// itoa converts an int to a string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}
