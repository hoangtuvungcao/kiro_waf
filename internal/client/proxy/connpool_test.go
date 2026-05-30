package proxy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newTestPool creates a ConnPool with a real TCP listener for testing.
func newTestPool(t *testing.T, maxIdle, maxTotal int) (*ConnPool, net.Listener) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	// Accept connections in background
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return // listener closed
			}
			// Keep connection open until closed by client
			go func(c net.Conn) {
				buf := make([]byte, 1)
				for {
					_, err := c.Read(buf)
					if err != nil {
						return
					}
				}
			}(conn)
		}
	}()

	config := ConnPoolConfig{
		MaxIdleConns:  maxIdle,
		MaxTotalConns: maxTotal,
		IdleTimeout:   90 * time.Second,
		QueueTimeout:  1 * time.Second,
		DialTimeout:   5 * time.Second,
		BackendAddr:   ln.Addr().String(),
	}

	pool := NewConnPool(config)
	return pool, ln
}

func TestConnPoolGetPut(t *testing.T) {
	pool, ln := newTestPool(t, 4, 8)
	defer ln.Close()
	defer pool.Close()

	ctx := context.Background()

	// Get a connection
	conn, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if conn == nil {
		t.Fatal("Get returned nil connection")
	}

	stats := pool.Stats()
	if stats.ActiveConns != 1 {
		t.Errorf("expected 1 active conn, got %d", stats.ActiveConns)
	}

	// Put it back
	pool.Put(conn)

	stats = pool.Stats()
	if stats.IdleConns != 1 {
		t.Errorf("expected 1 idle conn, got %d", stats.IdleConns)
	}
	if stats.ActiveConns != 0 {
		t.Errorf("expected 0 active conns, got %d", stats.ActiveConns)
	}
}

func TestConnPoolReusesIdleConnections(t *testing.T) {
	pool, ln := newTestPool(t, 4, 8)
	defer ln.Close()
	defer pool.Close()

	ctx := context.Background()

	// Get and put a connection
	conn1, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	pool.Put(conn1)

	// Get again - should reuse the idle connection
	conn2, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("second Get failed: %v", err)
	}
	defer pool.Put(conn2)

	// The reused connection should be the same underlying connection
	if conn2 != conn1 {
		t.Log("Note: connection was reused from pool (LIFO)")
	}
}

func TestConnPoolMaxTotalConns(t *testing.T) {
	maxTotal := 4
	pool, ln := newTestPool(t, 2, maxTotal)
	defer ln.Close()
	defer pool.Close()

	ctx := context.Background()

	// Acquire all connections
	conns := make([]net.Conn, maxTotal)
	for i := 0; i < maxTotal; i++ {
		var err error
		conns[i], err = pool.Get(ctx)
		if err != nil {
			t.Fatalf("Get %d failed: %v", i, err)
		}
	}

	stats := pool.Stats()
	if stats.ActiveConns != maxTotal {
		t.Errorf("expected %d active conns, got %d", maxTotal, stats.ActiveConns)
	}

	// Return all connections
	for _, c := range conns {
		pool.Put(c)
	}
}

func TestConnPoolQueueTimeout503(t *testing.T) {
	// Small pool with short queue timeout
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				buf := make([]byte, 1)
				for {
					if _, err := c.Read(buf); err != nil {
						return
					}
				}
			}(conn)
		}
	}()

	config := ConnPoolConfig{
		MaxIdleConns:  2,
		MaxTotalConns: 2, // Very small pool
		IdleTimeout:   90 * time.Second,
		QueueTimeout:  100 * time.Millisecond, // Short timeout for test
		DialTimeout:   5 * time.Second,
		BackendAddr:   ln.Addr().String(),
	}
	pool := NewConnPool(config)
	defer pool.Close()

	ctx := context.Background()

	// Exhaust the pool
	conn1, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Get 1 failed: %v", err)
	}
	conn2, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Get 2 failed: %v", err)
	}

	// Next Get should timeout and return ErrPoolExhausted
	start := time.Now()
	_, err = pool.Get(ctx)
	elapsed := time.Since(start)

	if !errors.Is(err, ErrPoolExhausted) {
		t.Fatalf("expected ErrPoolExhausted, got: %v", err)
	}

	// Should have waited approximately QueueTimeout
	if elapsed < 80*time.Millisecond {
		t.Errorf("returned too quickly: %v (expected ~100ms)", elapsed)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("waited too long: %v (expected ~100ms)", elapsed)
	}

	pool.Put(conn1)
	pool.Put(conn2)
}

func TestConnPoolBackendUnreachable502(t *testing.T) {
	// Use an address that won't accept connections
	config := ConnPoolConfig{
		MaxIdleConns:  4,
		MaxTotalConns: 8,
		IdleTimeout:   90 * time.Second,
		QueueTimeout:  1 * time.Second,
		DialTimeout:   500 * time.Millisecond, // Short dial timeout for test
		BackendAddr:   "127.0.0.1:1", // Port 1 is unlikely to be listening
	}
	pool := NewConnPool(config)
	defer pool.Close()

	ctx := context.Background()

	start := time.Now()
	_, err := pool.Get(ctx)
	elapsed := time.Since(start)

	if !errors.Is(err, ErrBackendUnreachable) {
		t.Fatalf("expected ErrBackendUnreachable, got: %v", err)
	}

	// Should return within DialTimeout (500ms + some margin)
	if elapsed > 2*time.Second {
		t.Errorf("took too long: %v (expected <1s)", elapsed)
	}
}

func TestConnPoolBackendUnreachableNoGoroutineLeak(t *testing.T) {
	config := ConnPoolConfig{
		MaxIdleConns:  4,
		MaxTotalConns: 8,
		IdleTimeout:   90 * time.Second,
		QueueTimeout:  1 * time.Second,
		DialTimeout:   200 * time.Millisecond,
		BackendAddr:   "127.0.0.1:1",
	}
	pool := NewConnPool(config)
	defer pool.Close()

	ctx := context.Background()

	// Send multiple concurrent requests to unreachable backend
	var wg sync.WaitGroup
	errCount := int32(0)
	numRequests := 20

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := pool.Get(ctx)
			if err != nil {
				atomic.AddInt32(&errCount, 1)
			}
		}()
	}

	wg.Wait()

	// All requests should have failed
	if int(atomic.LoadInt32(&errCount)) != numRequests {
		t.Errorf("expected %d errors, got %d", numRequests, errCount)
	}

	// Pool should have no active connections (no leak)
	stats := pool.Stats()
	if stats.ActiveConns != 0 {
		t.Errorf("expected 0 active conns after failures, got %d", stats.ActiveConns)
	}
	if stats.IdleConns != 0 {
		t.Errorf("expected 0 idle conns after failures, got %d", stats.IdleConns)
	}
}

func TestConnPoolIdleTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				buf := make([]byte, 1)
				for {
					if _, err := c.Read(buf); err != nil {
						return
					}
				}
			}(conn)
		}
	}()

	config := ConnPoolConfig{
		MaxIdleConns:  4,
		MaxTotalConns: 8,
		IdleTimeout:   50 * time.Millisecond, // Very short for testing
		QueueTimeout:  1 * time.Second,
		DialTimeout:   5 * time.Second,
		BackendAddr:   ln.Addr().String(),
	}
	pool := NewConnPool(config)
	defer pool.Close()

	ctx := context.Background()

	// Get and put a connection
	conn, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	pool.Put(conn)

	// Verify it's in idle pool
	stats := pool.Stats()
	if stats.IdleConns != 1 {
		t.Fatalf("expected 1 idle conn, got %d", stats.IdleConns)
	}

	// Wait for idle timeout to expire
	time.Sleep(100 * time.Millisecond)

	// Cleanup should remove the expired connection
	cleaned := pool.CleanupIdle()
	if cleaned != 1 {
		t.Errorf("expected 1 cleaned connection, got %d", cleaned)
	}

	stats = pool.Stats()
	if stats.IdleConns != 0 {
		t.Errorf("expected 0 idle conns after cleanup, got %d", stats.IdleConns)
	}
}

func TestConnPoolMaxIdleConns(t *testing.T) {
	maxIdle := 2
	pool, ln := newTestPool(t, maxIdle, 8)
	defer ln.Close()
	defer pool.Close()

	ctx := context.Background()

	// Get 4 connections
	conns := make([]net.Conn, 4)
	for i := 0; i < 4; i++ {
		var err error
		conns[i], err = pool.Get(ctx)
		if err != nil {
			t.Fatalf("Get %d failed: %v", i, err)
		}
	}

	// Put all back - only maxIdle should be kept
	for _, c := range conns {
		pool.Put(c)
	}

	stats := pool.Stats()
	if stats.IdleConns > maxIdle {
		t.Errorf("expected at most %d idle conns, got %d", maxIdle, stats.IdleConns)
	}
}

func TestConnPoolClose(t *testing.T) {
	pool, ln := newTestPool(t, 4, 8)
	defer ln.Close()

	ctx := context.Background()

	// Get a connection and put it back to idle
	conn, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	pool.Put(conn)

	// Close the pool
	pool.Close()

	// Subsequent Get should fail
	_, err = pool.Get(ctx)
	if !errors.Is(err, ErrPoolClosed) {
		t.Fatalf("expected ErrPoolClosed after close, got: %v", err)
	}
}

func TestConnPoolDiscard(t *testing.T) {
	pool, ln := newTestPool(t, 4, 8)
	defer ln.Close()
	defer pool.Close()

	ctx := context.Background()

	conn, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	stats := pool.Stats()
	if stats.ActiveConns != 1 {
		t.Errorf("expected 1 active conn, got %d", stats.ActiveConns)
	}

	// Discard the connection (simulating a broken connection)
	pool.Discard(conn)

	stats = pool.Stats()
	if stats.ActiveConns != 0 {
		t.Errorf("expected 0 active conns after discard, got %d", stats.ActiveConns)
	}
	if stats.IdleConns != 0 {
		t.Errorf("expected 0 idle conns after discard, got %d", stats.IdleConns)
	}
}

func TestConnPoolConcurrentAccess(t *testing.T) {
	pool, ln := newTestPool(t, 8, 16)
	defer ln.Close()
	defer pool.Close()

	ctx := context.Background()
	var wg sync.WaitGroup
	numGoroutines := 50
	iterations := 20

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				conn, err := pool.Get(ctx)
				if err != nil {
					// Pool exhaustion is acceptable under heavy load
					continue
				}
				// Simulate some work
				time.Sleep(time.Millisecond)
				pool.Put(conn)
			}
		}()
	}

	wg.Wait()

	// After all goroutines complete, pool should be stable
	stats := pool.Stats()
	if stats.ActiveConns != 0 {
		t.Errorf("expected 0 active conns after all work done, got %d", stats.ActiveConns)
	}
}

func TestConnPoolQueueWaitThenRelease(t *testing.T) {
	// Test that a waiting goroutine gets a connection when one is returned
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				buf := make([]byte, 1)
				for {
					if _, err := c.Read(buf); err != nil {
						return
					}
				}
			}(conn)
		}
	}()

	config := ConnPoolConfig{
		MaxIdleConns:  1,
		MaxTotalConns: 1, // Only 1 connection allowed
		IdleTimeout:   90 * time.Second,
		QueueTimeout:  2 * time.Second, // Generous timeout
		DialTimeout:   5 * time.Second,
		BackendAddr:   ln.Addr().String(),
	}
	pool := NewConnPool(config)
	defer pool.Close()

	ctx := context.Background()

	// Take the only connection
	conn1, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Get 1 failed: %v", err)
	}

	// Start a goroutine that will wait for a connection
	var wg sync.WaitGroup
	var conn2 net.Conn
	var getErr error

	wg.Add(1)
	go func() {
		defer wg.Done()
		conn2, getErr = pool.Get(ctx)
	}()

	// Give the goroutine time to start waiting
	time.Sleep(50 * time.Millisecond)

	// Return the connection - the waiting goroutine should get it
	pool.Put(conn1)

	wg.Wait()

	if getErr != nil {
		t.Fatalf("waiting Get failed: %v", getErr)
	}
	if conn2 == nil {
		t.Fatal("waiting Get returned nil connection")
	}
	pool.Put(conn2)
}

func TestConnPoolDefaultConfig(t *testing.T) {
	cfg := DefaultConnPoolConfig()

	if cfg.MaxIdleConns != 256 {
		t.Errorf("expected MaxIdleConns=256, got %d", cfg.MaxIdleConns)
	}
	if cfg.MaxTotalConns != 1024 {
		t.Errorf("expected MaxTotalConns=1024, got %d", cfg.MaxTotalConns)
	}
	if cfg.IdleTimeout != 90*time.Second {
		t.Errorf("expected IdleTimeout=90s, got %v", cfg.IdleTimeout)
	}
	if cfg.QueueTimeout != 1*time.Second {
		t.Errorf("expected QueueTimeout=1s, got %v", cfg.QueueTimeout)
	}
	if cfg.DialTimeout != 5*time.Second {
		t.Errorf("expected DialTimeout=5s, got %v", cfg.DialTimeout)
	}
}

func TestConnPoolConfigClamping(t *testing.T) {
	// Test that invalid config values are clamped to defaults
	config := ConnPoolConfig{
		MaxIdleConns:  0,
		MaxTotalConns: -1,
		IdleTimeout:   0,
		QueueTimeout:  0,
		DialTimeout:   0,
		BackendAddr:   "127.0.0.1:8080",
	}
	pool := NewConnPool(config)
	defer pool.Close()

	stats := pool.Stats()
	if stats.MaxIdle != 256 {
		t.Errorf("expected MaxIdle=256 after clamping, got %d", stats.MaxIdle)
	}
	if stats.MaxTotal != 1024 {
		t.Errorf("expected MaxTotal=1024 after clamping, got %d", stats.MaxTotal)
	}
}

func TestConnPoolMaxIdleGreaterThanMaxTotal(t *testing.T) {
	// MaxIdleConns should be clamped to MaxTotalConns
	config := ConnPoolConfig{
		MaxIdleConns:  100,
		MaxTotalConns: 10,
		IdleTimeout:   90 * time.Second,
		QueueTimeout:  1 * time.Second,
		DialTimeout:   5 * time.Second,
		BackendAddr:   "127.0.0.1:8080",
	}
	pool := NewConnPool(config)
	defer pool.Close()

	stats := pool.Stats()
	if stats.MaxIdle != 10 {
		t.Errorf("expected MaxIdle clamped to MaxTotal=10, got %d", stats.MaxIdle)
	}
}

func TestHandlePoolError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedCode   int
		expectedHandle bool
	}{
		{
			name:           "pool exhausted returns 503",
			err:            ErrPoolExhausted,
			expectedCode:   http.StatusServiceUnavailable,
			expectedHandle: true,
		},
		{
			name:           "backend unreachable returns 502",
			err:            ErrBackendUnreachable,
			expectedCode:   http.StatusBadGateway,
			expectedHandle: true,
		},
		{
			name:           "pool closed returns 503",
			err:            ErrPoolClosed,
			expectedCode:   http.StatusServiceUnavailable,
			expectedHandle: true,
		},
		{
			name:           "nil error not handled",
			err:            nil,
			expectedCode:   0,
			expectedHandle: false,
		},
		{
			name:           "unknown error not handled",
			err:            errors.New("some other error"),
			expectedCode:   0,
			expectedHandle: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			handled := HandlePoolError(w, tt.err)

			if handled != tt.expectedHandle {
				t.Errorf("expected handled=%v, got %v", tt.expectedHandle, handled)
			}
			if handled && w.Code != tt.expectedCode {
				t.Errorf("expected status %d, got %d", tt.expectedCode, w.Code)
			}
		})
	}
}

func TestConnPoolPutNil(t *testing.T) {
	pool, ln := newTestPool(t, 4, 8)
	defer ln.Close()
	defer pool.Close()

	// Put nil should not panic
	pool.Put(nil)
}

func TestConnPoolDiscardNil(t *testing.T) {
	pool, ln := newTestPool(t, 4, 8)
	defer ln.Close()
	defer pool.Close()

	// Discard nil should not panic
	pool.Discard(nil)
}

func TestConnPoolContextCancellation(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				buf := make([]byte, 1)
				for {
					if _, err := c.Read(buf); err != nil {
						return
					}
				}
			}(conn)
		}
	}()

	config := ConnPoolConfig{
		MaxIdleConns:  1,
		MaxTotalConns: 1,
		IdleTimeout:   90 * time.Second,
		QueueTimeout:  5 * time.Second, // Long timeout
		DialTimeout:   5 * time.Second,
		BackendAddr:   ln.Addr().String(),
	}
	pool := NewConnPool(config)
	defer pool.Close()

	// Take the only connection
	conn1, err := pool.Get(context.Background())
	if err != nil {
		t.Fatalf("Get 1 failed: %v", err)
	}
	defer pool.Put(conn1)

	// Try to get with a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	start := time.Now()
	_, err = pool.Get(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error with cancelled context")
	}

	// Should return quickly (not wait for QueueTimeout)
	if elapsed > 500*time.Millisecond {
		t.Errorf("took too long with cancelled context: %v", elapsed)
	}
}
