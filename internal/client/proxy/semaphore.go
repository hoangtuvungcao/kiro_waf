package proxy

import (
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

const (
	// DefaultMaxGoroutines is the default maximum number of concurrent handler goroutines.
	DefaultMaxGoroutines = 10000

	// DefaultReadTimeout is the default read timeout for HTTP connections.
	DefaultReadTimeout = 30 * time.Second

	// DefaultWriteTimeout is the default write timeout for HTTP connections.
	DefaultWriteTimeout = 60 * time.Second
)

// Semaphore limits concurrent goroutines using a channel-based pattern.
// When the channel is full (at capacity), Acquire returns false immediately,
// signaling that the request should be rejected with HTTP 503.
type Semaphore struct {
	ch chan struct{}
}

// NewSemaphore creates a Semaphore with the given maximum concurrency.
// If max <= 0, DefaultMaxGoroutines is used.
func NewSemaphore(max int) *Semaphore {
	if max <= 0 {
		max = DefaultMaxGoroutines
	}
	return &Semaphore{ch: make(chan struct{}, max)}
}

// Acquire attempts to acquire a semaphore slot without blocking.
// Returns true if a slot was acquired (caller must call Release when done).
// Returns false if the semaphore is at capacity (caller should return 503).
func (s *Semaphore) Acquire() bool {
	select {
	case s.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

// Release releases a previously acquired semaphore slot.
// Must be called exactly once for each successful Acquire.
func (s *Semaphore) Release() {
	<-s.ch
}

// Len returns the number of currently acquired slots.
func (s *Semaphore) Len() int {
	return len(s.ch)
}

// Cap returns the maximum capacity of the semaphore.
func (s *Semaphore) Cap() int {
	return cap(s.ch)
}

// SemaphoreMiddleware wraps an http.Handler and limits concurrent request
// processing using the provided Semaphore. When the semaphore is at capacity,
// requests are immediately rejected with HTTP 503 Service Unavailable.
func SemaphoreMiddleware(sem *Semaphore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !sem.Acquire() {
			http.Error(w, "Service Unavailable: goroutine limit reached", http.StatusServiceUnavailable)
			return
		}
		defer sem.Release()
		next.ServeHTTP(w, r)
	})
}

// RecoverMiddleware wraps an http.Handler and recovers from panics in downstream
// handlers. When a panic occurs, it logs the panic value and full stack trace,
// then returns HTTP 500 Internal Server Error. The server continues serving
// subsequent requests without process termination.
func RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()
				log.Printf("PANIC recovered: %v\n%s", err, stack)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ServerConfig holds HTTP server timeout configuration.
// These timeouts prevent resource exhaustion from slow or malicious clients.
type ServerConfig struct {
	// ReadTimeout is the maximum duration for reading the entire request,
	// including the body. Default: 30s.
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out writes of the
	// response. Default: 60s.
	WriteTimeout time.Duration

	// IdleTimeout is the maximum amount of time to wait for the next request
	// when keep-alives are enabled. Default: 120s.
	IdleTimeout time.Duration
}

// DefaultServerConfig returns a ServerConfig with the standard WAF timeouts.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		ReadTimeout:  DefaultReadTimeout,
		WriteTimeout: DefaultWriteTimeout,
		IdleTimeout:  120 * time.Second,
	}
}

// ApplyTo configures the given http.Server with the timeout values from this config.
// This should be called before starting the server.
func (c ServerConfig) ApplyTo(srv *http.Server) {
	if c.ReadTimeout > 0 {
		srv.ReadTimeout = c.ReadTimeout
	}
	if c.WriteTimeout > 0 {
		srv.WriteTimeout = c.WriteTimeout
	}
	if c.IdleTimeout > 0 {
		srv.IdleTimeout = c.IdleTimeout
	}
}

// NewConfiguredServer creates an http.Server with the standard WAF timeouts applied.
// This is a convenience function for creating servers with proper timeout configuration.
func NewConfiguredServer(addr string, handler http.Handler) *http.Server {
	cfg := DefaultServerConfig()
	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}
	cfg.ApplyTo(srv)
	return srv
}
