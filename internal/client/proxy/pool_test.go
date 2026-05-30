package proxy

import (
	"net/http"
	"net/url"
	"testing"
)

// TestBufferPoolGetPut verifies basic pool get/put cycle.
func TestBufferPoolGetPut(t *testing.T) {
	pool := NewBufferPool(DefaultBufferSize)

	buf := pool.Get()
	if buf == nil {
		t.Fatal("Get returned nil")
	}
	if len(*buf) != DefaultBufferSize {
		t.Fatalf("expected buffer length %d, got %d", DefaultBufferSize, len(*buf))
	}
	if cap(*buf) < DefaultBufferSize {
		t.Fatalf("expected buffer capacity >= %d, got %d", DefaultBufferSize, cap(*buf))
	}

	// Write some data
	(*buf)[0] = 0xFF
	(*buf)[DefaultBufferSize-1] = 0xAA

	// Return to pool
	pool.Put(buf)

	// Get again - should get a buffer of correct size
	buf2 := pool.Get()
	if len(*buf2) != DefaultBufferSize {
		t.Fatalf("after put/get, expected length %d, got %d", DefaultBufferSize, len(*buf2))
	}
	pool.Put(buf2)
}

// TestBufferPoolOversizedDiscard verifies oversized buffers are discarded.
func TestBufferPoolOversizedDiscard(t *testing.T) {
	pool := NewBufferPool(SmallBufferSize)

	// Simulate a buffer that grew beyond MaxBufferSize
	oversized := make([]byte, MaxBufferSize+1)
	pool.Put(&oversized)

	// Next Get should return a fresh buffer of the correct size
	buf := pool.Get()
	if len(*buf) != SmallBufferSize {
		t.Fatalf("expected fresh buffer of size %d, got %d", SmallBufferSize, len(*buf))
	}
	pool.Put(buf)
}

// TestBufferPoolNilPut verifies nil put doesn't panic.
func TestBufferPoolNilPut(t *testing.T) {
	pool := NewBufferPool(DefaultBufferSize)
	pool.Put(nil) // should not panic
}

// TestBufferPoolSizeClamping verifies size bounds.
func TestBufferPoolSizeClamping(t *testing.T) {
	// Zero size should use default
	pool := NewBufferPool(0)
	if pool.Size() != DefaultBufferSize {
		t.Fatalf("expected default size %d for zero input, got %d", DefaultBufferSize, pool.Size())
	}

	// Negative size should use default
	pool = NewBufferPool(-1)
	if pool.Size() != DefaultBufferSize {
		t.Fatalf("expected default size %d for negative input, got %d", DefaultBufferSize, pool.Size())
	}

	// Oversized should be clamped to MaxBufferSize
	pool = NewBufferPool(MaxBufferSize + 100)
	if pool.Size() != MaxBufferSize {
		t.Fatalf("expected max size %d for oversized input, got %d", MaxBufferSize, pool.Size())
	}
}

// TestInspectHeaders verifies header inspection logic.
func TestInspectHeaders(t *testing.T) {
	tests := []struct {
		name          string
		headers       http.Header
		expectedFlags uint8
	}{
		{
			name:          "empty headers",
			headers:       http.Header{},
			expectedFlags: 0,
		},
		{
			name: "upgrade connection",
			headers: http.Header{
				"Connection": []string{"upgrade"},
			},
			expectedFlags: FlagWebSocket,
		},
		{
			name: "keep-alive connection",
			headers: http.Header{
				"Connection": []string{"keep-alive"},
			},
			expectedFlags: FlagKeepAlive,
		},
		{
			name: "has content-length body",
			headers: http.Header{
				"Content-Length": []string{"1024"},
			},
			expectedFlags: FlagHasBody,
		},
		{
			name: "zero content-length no body",
			headers: http.Header{
				"Content-Length": []string{"0"},
			},
			expectedFlags: 0,
		},
		{
			name: "chunked transfer-encoding",
			headers: http.Header{
				"Transfer-Encoding": []string{"chunked"},
			},
			expectedFlags: FlagHasBody,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &http.Request{Header: tt.headers}
			var decision RouteDecision
			InspectHeaders(r, &decision)
			if decision.Flags != tt.expectedFlags {
				t.Errorf("expected flags %d, got %d", tt.expectedFlags, decision.Flags)
			}
			if decision.Action != RouteActionProxy {
				t.Errorf("expected action %d, got %d", RouteActionProxy, decision.Action)
			}
		})
	}
}

// TestInspectClientIP verifies zero-allocation IP extraction.
func TestInspectClientIP(t *testing.T) {
	tests := []struct {
		name     string
		xff      string
		expected string
	}{
		{"no header", "", ""},
		{"single IP", "192.168.1.1", "192.168.1.1"},
		{"multiple IPs", "10.0.0.1, 192.168.1.1, 172.16.0.1", "10.0.0.1"},
		{"with spaces", "  10.0.0.1  , 192.168.1.1", "10.0.0.1"},
		{"IPv6", "2001:db8::1, 10.0.0.1", "2001:db8::1"},
	}

	buf := make([]byte, 64)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &http.Request{Header: http.Header{}}
			if tt.xff != "" {
				r.Header["X-Forwarded-For"] = []string{tt.xff}
			}
			n := InspectClientIP(r, buf)
			got := string(buf[:n])
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

// TestMatchRoute verifies path-based routing.
func TestMatchRoute(t *testing.T) {
	tests := []struct {
		path           string
		expectedAction uint8
	}{
		{"/", RouteActionProxy},
		{"/api/v1/users", RouteActionProxy},
		{"/__kiro/challenge/verify", RouteActionChallenge},
		{"/__kiro/hold/verify", RouteActionChallenge},
		{"/healthz", RouteActionProxy},
		{"", RouteActionProxy},
	}

	var decision RouteDecision
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			MatchRoute(tt.path, &decision)
			if decision.Action != tt.expectedAction {
				t.Errorf("path %q: expected action %d, got %d", tt.path, tt.expectedAction, decision.Action)
			}
		})
	}
}

// TestGCConfigDefaults verifies default GC configuration values.
func TestGCConfigDefaults(t *testing.T) {
	// Clear env vars for this test
	t.Setenv("GOGC", "")
	t.Setenv("GOMEMLIMIT", "")

	cfg := ConfigureGC()
	if cfg.GOGC != DefaultGOGC {
		t.Errorf("expected GOGC=%d, got %d", DefaultGOGC, cfg.GOGC)
	}
	if cfg.GOMEMLIMIT != DefaultGOMEMLIMIT {
		t.Errorf("expected GOMEMLIMIT=%d, got %d", DefaultGOMEMLIMIT, cfg.GOMEMLIMIT)
	}
}

// TestGCConfigFromEnv verifies GC configuration from environment.
func TestGCConfigFromEnv(t *testing.T) {
	t.Setenv("GOGC", "100")
	t.Setenv("GOMEMLIMIT", "1GiB")

	cfg := ConfigureGC()
	if cfg.GOGC != 100 {
		t.Errorf("expected GOGC=100, got %d", cfg.GOGC)
	}
	expected := int64(1024 * 1024 * 1024)
	if cfg.GOMEMLIMIT != expected {
		t.Errorf("expected GOMEMLIMIT=%d, got %d", expected, cfg.GOMEMLIMIT)
	}
}

// TestParseMemLimit verifies memory limit parsing.
func TestParseMemLimit(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		wantErr  bool
	}{
		{"512", 512, false},
		{"512B", 512, false},
		{"1KB", 1000, false},
		{"1KiB", 1024, false},
		{"512MB", 512_000_000, false},
		{"512MiB", 512 * 1024 * 1024, false},
		{"1GB", 1_000_000_000, false},
		{"1GiB", 1024 * 1024 * 1024, false},
		{"", 0, true},
		{"abc", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseMemLimit(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for input %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tt.input, err)
			}
			if got != tt.expected {
				t.Errorf("input %q: expected %d, got %d", tt.input, tt.expected, got)
			}
		})
	}
}

// BenchmarkBufferPoolGetPut benchmarks the pool get/put cycle.
// This should report 0 allocs/op after warmup since buffers are reused.
func BenchmarkBufferPoolGetPut(b *testing.B) {
	pool := NewBufferPool(DefaultBufferSize)

	// Warmup: pre-populate the pool
	buf := pool.Get()
	pool.Put(buf)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		pool.Put(buf)
	}
}

// BenchmarkInspectHeaders benchmarks the zero-allocation header inspection.
// This MUST report 0 allocs/op to satisfy Requirement 7.4.
func BenchmarkInspectHeaders(b *testing.B) {
	r := &http.Request{
		Header: http.Header{
			"Connection":        []string{"keep-alive"},
			"Content-Length":    []string{"1024"},
			"Host":             []string{"example.com"},
			"User-Agent":       []string{"Mozilla/5.0"},
			"Accept":           []string{"text/html"},
			"X-Forwarded-For":  []string{"192.168.1.100"},
			"Transfer-Encoding": []string{""},
		},
	}

	var decision RouteDecision

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		InspectHeaders(r, &decision)
	}
}

// BenchmarkInspectClientIP benchmarks zero-allocation IP extraction.
// This MUST report 0 allocs/op.
func BenchmarkInspectClientIP(b *testing.B) {
	r := &http.Request{
		Header: http.Header{
			"X-Forwarded-For": []string{"10.0.0.1, 192.168.1.1, 172.16.0.1"},
		},
	}
	buf := make([]byte, 64)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		InspectClientIP(r, buf)
	}
}

// BenchmarkMatchRoute benchmarks zero-allocation path routing.
// This MUST report 0 allocs/op.
func BenchmarkMatchRoute(b *testing.B) {
	var decision RouteDecision
	path := "/api/v1/users/profile"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		MatchRoute(path, &decision)
	}
}

// BenchmarkHotPath benchmarks the complete hot path: header inspection + routing + IP extraction.
// This is the combined benchmark that MUST report 0 allocs/op per Requirement 7.4.
func BenchmarkHotPath(b *testing.B) {
	r := &http.Request{
		Header: http.Header{
			"Connection":       []string{"keep-alive"},
			"Content-Length":   []string{"512"},
			"Host":            []string{"example.com"},
			"User-Agent":      []string{"Mozilla/5.0 (X11; Linux x86_64)"},
			"Accept":          []string{"text/html,application/xhtml+xml"},
			"X-Forwarded-For": []string{"203.0.113.50, 198.51.100.1"},
		},
		RequestURI: "/api/v1/data",
	}
	r.URL = &url.URL{Path: "/api/v1/data"}

	var decision RouteDecision
	ipBuf := make([]byte, 64)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Step 1: Inspect headers (zero-alloc)
		InspectHeaders(r, &decision)

		// Step 2: Extract client IP (zero-alloc)
		InspectClientIP(r, ipBuf)

		// Step 3: Route matching (zero-alloc)
		MatchRoute(r.URL.Path, &decision)
	}
}
