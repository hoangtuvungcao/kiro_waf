// Package proxy provides high-performance proxy components for the WAF.
// BufferPool uses sync.Pool to reuse byte buffers for request/response copying,
// reducing GC pressure and achieving zero-allocation in the hot path.
package proxy

import (
	"sync"
)

const (
	// DefaultBufferSize is the default buffer size for request/response copying (32KB).
	DefaultBufferSize = 32 * 1024

	// SmallBufferSize is used for header inspection buffers (4KB).
	SmallBufferSize = 4 * 1024

	// MaxBufferSize caps buffer growth to prevent memory waste (256KB).
	MaxBufferSize = 256 * 1024
)

// BufferPool manages reusable byte buffers via sync.Pool to minimize
// heap allocations in the proxy forwarding path.
type BufferPool struct {
	pool sync.Pool
	size int
}

// NewBufferPool creates a BufferPool that produces buffers of the given size.
// If size <= 0, DefaultBufferSize is used.
func NewBufferPool(size int) *BufferPool {
	if size <= 0 {
		size = DefaultBufferSize
	}
	if size > MaxBufferSize {
		size = MaxBufferSize
	}
	bp := &BufferPool{size: size}
	bp.pool = sync.Pool{
		New: func() any {
			buf := make([]byte, bp.size)
			return &buf
		},
	}
	return bp
}

// Get retrieves a buffer from the pool. The returned buffer has length equal
// to the pool's configured size. Callers must call Put when done.
func (bp *BufferPool) Get() *[]byte {
	return bp.pool.Get().(*[]byte)
}

// Put returns a buffer to the pool for reuse. Buffers that have grown beyond
// MaxBufferSize are discarded to prevent memory bloat.
func (bp *BufferPool) Put(buf *[]byte) {
	if buf == nil {
		return
	}
	// Discard oversized buffers to prevent memory bloat
	if cap(*buf) > MaxBufferSize {
		return
	}
	// Reset length to pool size for next user
	*buf = (*buf)[:bp.size]
	bp.pool.Put(buf)
}

// Size returns the configured buffer size.
func (bp *BufferPool) Size() int {
	return bp.size
}

// HeaderBufferPool is a specialized pool for small buffers used in header inspection.
// These are smaller (4KB) since headers are typically compact.
var HeaderBufferPool = NewBufferPool(SmallBufferSize)

// CopyBufferPool is the default pool for request/response body copying (32KB).
var CopyBufferPool = NewBufferPool(DefaultBufferSize)
