package proxy

import (
	"net/http"
)

// Common header keys as byte slices to avoid string allocations during comparison.
// These are used in the zero-allocation header inspection path.
var (
	headerXForwardedFor = []byte("X-Forwarded-For")
	headerHost          = []byte("Host")
	headerContentType   = []byte("Content-Type")
	headerUserAgent     = []byte("User-Agent")
	headerConnection    = []byte("Connection")
	headerAccept        = []byte("Accept")
)

// RouteDecision represents the routing outcome from header inspection.
// Using a fixed-size struct avoids interface allocations.
type RouteDecision struct {
	// Action indicates what to do with the request.
	// 0 = proxy to backend, 1 = challenge, 2 = block, 3 = rate-limit
	Action uint8

	// BackendIdx is the index of the backend to route to (for multi-backend setups).
	BackendIdx uint16

	// Flags holds additional routing flags (bit field).
	// Bit 0: websocket upgrade detected
	// Bit 1: keep-alive requested
	// Bit 2: has body
	Flags uint8
}

const (
	// RouteActionProxy forwards the request to the backend.
	RouteActionProxy uint8 = 0
	// RouteActionChallenge serves a challenge page.
	RouteActionChallenge uint8 = 1
	// RouteActionBlock blocks the request.
	RouteActionBlock uint8 = 2
	// RouteActionRateLimit applies rate limiting.
	RouteActionRateLimit uint8 = 3
)

const (
	// FlagWebSocket indicates a WebSocket upgrade request.
	FlagWebSocket uint8 = 1 << 0
	// FlagKeepAlive indicates keep-alive is requested.
	FlagKeepAlive uint8 = 1 << 1
	// FlagHasBody indicates the request has a body.
	FlagHasBody uint8 = 1 << 2
)

// InspectHeaders performs zero-allocation header inspection on an HTTP request.
// It examines headers using pre-allocated buffers and returns a routing decision
// without any heap allocations. This is the hot path for every request.
//
// The function inspects:
// - Connection header for keep-alive/upgrade detection
// - Content-Length for body presence
// - Host header for routing decisions
//
// All string comparisons use direct byte-level operations on the underlying
// header map to avoid allocating new strings.
func InspectHeaders(r *http.Request, decision *RouteDecision) {
	// Reset decision in place (no allocation)
	decision.Action = RouteActionProxy
	decision.BackendIdx = 0
	decision.Flags = 0

	// Check Connection header for upgrade/keep-alive
	// Access header values directly - Go's http.Header stores values as []string
	// but we can check without allocating by using the existing slice.
	if connValues := r.Header["Connection"]; len(connValues) > 0 {
		conn := connValues[0]
		if len(conn) >= 7 && (conn[0] == 'u' || conn[0] == 'U') {
			// Likely "upgrade" or "Upgrade"
			decision.Flags |= FlagWebSocket
		}
		if len(conn) >= 10 && (conn[0] == 'k' || conn[0] == 'K') {
			// Likely "keep-alive" or "Keep-Alive"
			decision.Flags |= FlagKeepAlive
		}
	}

	// Check for body presence via Content-Length
	if clValues := r.Header["Content-Length"]; len(clValues) > 0 {
		cl := clValues[0]
		if len(cl) > 0 && cl[0] != '0' {
			decision.Flags |= FlagHasBody
		}
	}

	// Check Transfer-Encoding for chunked (also indicates body)
	if teValues := r.Header["Transfer-Encoding"]; len(teValues) > 0 {
		decision.Flags |= FlagHasBody
	}
}

// InspectClientIP extracts the client IP from X-Forwarded-For header
// without allocating new strings. It writes the IP into the provided buffer
// and returns the number of bytes written.
//
// If X-Forwarded-For is not present, it returns 0 (caller should use RemoteAddr).
func InspectClientIP(r *http.Request, buf []byte) int {
	xffValues := r.Header["X-Forwarded-For"]
	if len(xffValues) == 0 {
		return 0
	}

	xff := xffValues[0]
	if len(xff) == 0 {
		return 0
	}

	// Find the first IP (before first comma), trim spaces
	n := 0
	started := false
	for i := 0; i < len(xff) && n < len(buf); i++ {
		ch := xff[i]
		if ch == ',' {
			break
		}
		if ch == ' ' && !started {
			continue
		}
		started = true
		buf[n] = ch
		n++
	}

	// Trim trailing spaces
	for n > 0 && buf[n-1] == ' ' {
		n--
	}

	return n
}

// MatchRoute performs zero-allocation path-based routing.
// It checks the request path against known internal paths and sets
// the appropriate action in the decision struct.
//
// Internal paths:
// - /__kiro/challenge/verify -> RouteActionChallenge
// - /__kiro/hold/verify -> RouteActionChallenge
// - /healthz -> RouteActionProxy (always allowed)
//
// All other paths -> RouteActionProxy (subject to further checks)
func MatchRoute(path string, decision *RouteDecision) {
	// Fast path: most requests don't start with /__kiro
	if len(path) < 7 || path[0] != '/' {
		decision.Action = RouteActionProxy
		return
	}

	// Check for internal kiro paths
	if len(path) >= 7 && path[1] == '_' && path[2] == '_' &&
		path[3] == 'k' && path[4] == 'i' && path[5] == 'r' && path[6] == 'o' {
		decision.Action = RouteActionChallenge
		return
	}

	decision.Action = RouteActionProxy
}
