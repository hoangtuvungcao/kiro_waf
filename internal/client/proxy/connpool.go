package proxy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Connection pool errors.
var (
	// ErrPoolExhausted is returned when the pool is at max capacity and the
	// queue timeout has been exceeded. Callers should return HTTP 503.
	ErrPoolExhausted = errors.New("connection pool exhausted: queue timeout exceeded")

	// ErrBackendUnreachable is returned when the backend cannot be reached
	// within the dial timeout. Callers should return HTTP 502.
	ErrBackendUnreachable = errors.New("backend unreachable")

	// ErrPoolClosed is returned when operations are attempted on a closed pool.
	ErrPoolClosed = errors.New("connection pool closed")
)

// ConnPoolConfig holds configuration for the backend connection pool.
type ConnPoolConfig struct {
	// MaxIdleConns is the maximum number of idle connections kept in the pool.
	// Default: 256
	MaxIdleConns int

	// MaxTotalConns is the maximum total number of connections (idle + active).
	// Default: 1024
	MaxTotalConns int

	// IdleTimeout is how long an idle connection can remain in the pool before
	// being closed. Default: 90s
	IdleTimeout time.Duration

	// QueueTimeout is the maximum time a Get() call will wait for a connection
	// when the pool is at max capacity. Default: 1s
	QueueTimeout time.Duration

	// DialTimeout is the maximum time to establish a new connection to the backend.
	// Default: 5s (ensures 502 within 5s when backend unreachable)
	DialTimeout time.Duration

	// BackendAddr is the address of the backend server (host:port).
	BackendAddr string
}

// DefaultConnPoolConfig returns a ConnPoolConfig with default values.
func DefaultConnPoolConfig() ConnPoolConfig {
	return ConnPoolConfig{
		MaxIdleConns:  256,
		MaxTotalConns: 1024,
		IdleTimeout:   90 * time.Second,
		QueueTimeout:  1 * time.Second,
		DialTimeout:   5 * time.Second,
	}
}

// poolConn wraps a net.Conn with metadata for pool management.
type poolConn struct {
	net.Conn
	createdAt time.Time
	idleSince time.Time
}

// ConnPool manages a pool of backend connections with configurable limits,
// idle timeout cleanup, and queue-based waiting when at capacity.
//
// Behavior:
//   - Get() returns an idle connection if available, or dials a new one if under MaxTotalConns.
//   - If at MaxTotalConns, Get() blocks up to QueueTimeout waiting for a connection to be returned.
//   - If QueueTimeout is exceeded, ErrPoolExhausted is returned (caller returns 503).
//   - If the backend is unreachable, ErrBackendUnreachable is returned within DialTimeout (caller returns 502).
//   - Put() returns a connection to the idle pool (up to MaxIdleConns) or closes it.
//   - CleanupIdle() removes connections that have been idle longer than IdleTimeout.
type ConnPool struct {
	config ConnPoolConfig

	mu       sync.Mutex
	idle     []*poolConn // idle connections available for reuse
	totalOut int32       // number of connections currently checked out (atomic)

	// semaphore limits total connections (idle + active)
	sem chan struct{}

	// waiterQueue allows Get() to wait for a connection when pool is full
	waiters int32 // atomic count of waiting goroutines

	closed atomic.Bool

	// dialer for creating new connections
	dialer net.Dialer
}

// NewConnPool creates a new connection pool with the given configuration.
// If config values are zero/invalid, defaults are applied.
func NewConnPool(config ConnPoolConfig) *ConnPool {
	if config.MaxIdleConns <= 0 {
		config.MaxIdleConns = 256
	}
	if config.MaxTotalConns <= 0 {
		config.MaxTotalConns = 1024
	}
	if config.MaxIdleConns > config.MaxTotalConns {
		config.MaxIdleConns = config.MaxTotalConns
	}
	if config.IdleTimeout <= 0 {
		config.IdleTimeout = 90 * time.Second
	}
	if config.QueueTimeout <= 0 {
		config.QueueTimeout = 1 * time.Second
	}
	if config.DialTimeout <= 0 {
		config.DialTimeout = 5 * time.Second
	}

	p := &ConnPool{
		config: config,
		idle:   make([]*poolConn, 0, config.MaxIdleConns),
		sem:    make(chan struct{}, config.MaxTotalConns),
		dialer: net.Dialer{
			Timeout: config.DialTimeout,
		},
	}

	return p
}

// Get retrieves a connection from the pool. It follows this logic:
//  1. If an idle connection is available, return it immediately.
//  2. If total connections < MaxTotalConns, dial a new connection.
//  3. If at MaxTotalConns, wait up to QueueTimeout for a connection.
//  4. If QueueTimeout exceeded, return ErrPoolExhausted (503).
//  5. If dial fails (backend unreachable), return ErrBackendUnreachable (502).
func (p *ConnPool) Get(ctx context.Context) (net.Conn, error) {
	if p.closed.Load() {
		return nil, ErrPoolClosed
	}

	// Try to acquire a semaphore slot (non-blocking first)
	select {
	case p.sem <- struct{}{}:
		// Got a slot - try idle connection first, then dial
		conn := p.getIdle()
		if conn != nil {
			return conn, nil
		}
		// Dial a new connection
		c, err := p.dial(ctx)
		if err != nil {
			// Release the semaphore slot since we failed to create a connection
			<-p.sem
			return nil, err
		}
		atomic.AddInt32(&p.totalOut, 1)
		return c, nil

	default:
		// Pool is at capacity - wait with timeout
		return p.waitForConn(ctx)
	}
}

// waitForConn blocks until a connection becomes available or QueueTimeout expires.
func (p *ConnPool) waitForConn(ctx context.Context) (net.Conn, error) {
	atomic.AddInt32(&p.waiters, 1)
	defer atomic.AddInt32(&p.waiters, -1)

	// Use the shorter of QueueTimeout and context deadline
	timeout := p.config.QueueTimeout
	deadline, hasDeadline := ctx.Deadline()
	if hasDeadline {
		remaining := time.Until(deadline)
		if remaining < timeout {
			timeout = remaining
		}
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case p.sem <- struct{}{}:
		// Got a slot
		conn := p.getIdle()
		if conn != nil {
			return conn, nil
		}
		// Dial new connection
		c, err := p.dial(ctx)
		if err != nil {
			<-p.sem
			return nil, err
		}
		atomic.AddInt32(&p.totalOut, 1)
		return c, nil

	case <-timer.C:
		return nil, ErrPoolExhausted

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// getIdle retrieves an idle connection from the pool (LIFO for better cache locality).
// Returns nil if no idle connections are available.
func (p *ConnPool) getIdle() net.Conn {
	p.mu.Lock()
	defer p.mu.Unlock()

	for len(p.idle) > 0 {
		// Pop from the end (LIFO - most recently used connection)
		n := len(p.idle) - 1
		pc := p.idle[n]
		p.idle[n] = nil // avoid memory leak
		p.idle = p.idle[:n]

		// Check if connection has exceeded idle timeout
		if time.Since(pc.idleSince) > p.config.IdleTimeout {
			pc.Conn.Close()
			continue
		}

		atomic.AddInt32(&p.totalOut, 1)
		return pc.Conn
	}
	return nil
}

// Put returns a connection to the pool for reuse. If the idle pool is full,
// the connection is closed. If the pool is closed, the connection is closed.
func (p *ConnPool) Put(conn net.Conn) {
	if conn == nil {
		return
	}

	atomic.AddInt32(&p.totalOut, -1)

	if p.closed.Load() {
		conn.Close()
		<-p.sem
		return
	}

	p.mu.Lock()
	if len(p.idle) < p.config.MaxIdleConns {
		p.idle = append(p.idle, &poolConn{
			Conn:      conn,
			createdAt: time.Now(),
			idleSince: time.Now(),
		})
		p.mu.Unlock()
	} else {
		p.mu.Unlock()
		conn.Close()
	}

	// Release semaphore slot to allow waiting goroutines to proceed
	<-p.sem
}

// Discard closes a connection and releases its slot without returning it to the pool.
// Use this when a connection is known to be broken.
func (p *ConnPool) Discard(conn net.Conn) {
	if conn == nil {
		return
	}
	atomic.AddInt32(&p.totalOut, -1)
	conn.Close()
	<-p.sem
}

// dial creates a new connection to the backend.
// Returns ErrBackendUnreachable if the connection cannot be established.
func (p *ConnPool) dial(ctx context.Context) (net.Conn, error) {
	// Create a context with dial timeout to ensure we return within DialTimeout
	dialCtx, cancel := context.WithTimeout(ctx, p.config.DialTimeout)
	defer cancel()

	conn, err := p.dialer.DialContext(dialCtx, "tcp", p.config.BackendAddr)
	if err != nil {
		return nil, ErrBackendUnreachable
	}
	return conn, nil
}

// CleanupIdle removes connections that have been idle longer than IdleTimeout.
// This should be called periodically (e.g., every 30s) to prevent stale connections.
func (p *ConnPool) CleanupIdle() int {
	if p.closed.Load() {
		return 0
	}

	p.mu.Lock()
	now := time.Now()
	cleaned := 0
	remaining := make([]*poolConn, 0, len(p.idle))

	for _, pc := range p.idle {
		if now.Sub(pc.idleSince) > p.config.IdleTimeout {
			pc.Conn.Close()
			cleaned++
		} else {
			remaining = append(remaining, pc)
		}
	}

	p.idle = remaining
	p.mu.Unlock()

	return cleaned
}

// Close shuts down the pool, closing all idle connections and preventing new ones.
// Active connections are not forcibly closed; they will be closed when returned via Put/Discard.
func (p *ConnPool) Close() {
	if !p.closed.CompareAndSwap(false, true) {
		return // already closed
	}

	p.mu.Lock()
	idle := p.idle
	p.idle = nil
	p.mu.Unlock()

	// Close idle connections. Idle connections do not hold semaphore slots
	// (slots are released in Put), so we just close the underlying connections.
	for _, pc := range idle {
		pc.Conn.Close()
	}
}

// Stats returns current pool statistics.
type PoolStats struct {
	// IdleConns is the number of idle connections in the pool.
	IdleConns int
	// ActiveConns is the number of connections currently checked out.
	ActiveConns int
	// TotalConns is idle + active connections.
	TotalConns int
	// Waiters is the number of goroutines waiting for a connection.
	Waiters int
	// MaxIdle is the configured maximum idle connections.
	MaxIdle int
	// MaxTotal is the configured maximum total connections.
	MaxTotal int
}

// Stats returns current pool statistics for monitoring.
func (p *ConnPool) Stats() PoolStats {
	p.mu.Lock()
	idleCount := len(p.idle)
	p.mu.Unlock()

	activeCount := int(atomic.LoadInt32(&p.totalOut))
	waitCount := int(atomic.LoadInt32(&p.waiters))

	return PoolStats{
		IdleConns:   idleCount,
		ActiveConns: activeCount,
		TotalConns:  idleCount + activeCount,
		Waiters:     waitCount,
		MaxIdle:     p.config.MaxIdleConns,
		MaxTotal:    p.config.MaxTotalConns,
	}
}

// HandlePoolError writes the appropriate HTTP error response based on the pool error.
// Returns true if the error was handled, false if it's not a pool error.
func HandlePoolError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}

	switch {
	case errors.Is(err, ErrPoolExhausted):
		http.Error(w, "Service Unavailable: connection pool exhausted", http.StatusServiceUnavailable)
		return true
	case errors.Is(err, ErrBackendUnreachable):
		http.Error(w, "Bad Gateway: backend unreachable", http.StatusBadGateway)
		return true
	case errors.Is(err, ErrPoolClosed):
		http.Error(w, "Service Unavailable: server shutting down", http.StatusServiceUnavailable)
		return true
	default:
		return false
	}
}
