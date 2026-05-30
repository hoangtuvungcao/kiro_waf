// Package monitor implements the Health Monitor subsystem for Kiro WAF Client Node.
// It continuously checks the health of XDP filter and the client binary,
// detects failures, and triggers self-recovery actions.
package monitor

import (
	"sync"
	"time"
)

// OperationMode represents the current operating mode of the Health Monitor.
type OperationMode string

const (
	// ModeOnline indicates normal operation with Master Server connectivity.
	ModeOnline OperationMode = "online"

	// ModeOffline indicates the client has lost connectivity to Master Server
	// and is operating with cached configuration.
	ModeOffline OperationMode = "offline"

	// ModeEmergency indicates the client is in emergency recovery mode
	// after a crash during high traffic, operating with stricter rate limits.
	ModeEmergency OperationMode = "emergency"
)

// MonitorStatus represents the current state of the Health Monitor.
type MonitorStatus struct {
	// Mode is the current operation mode (online, offline, emergency).
	Mode OperationMode `json:"mode"`

	// XDPAttached indicates whether the XDP program is attached to the network interface.
	XDPAttached bool `json:"xdp_attached"`

	// BinaryHealthy indicates whether the client binary is responding to health checks.
	BinaryHealthy bool `json:"binary_healthy"`

	// ConsecFailures is the number of consecutive health check failures.
	ConsecFailures int `json:"consec_failures"`

	// LastCheckAt is the timestamp of the last health check.
	LastCheckAt time.Time `json:"last_check_at"`

	// LastSnapshotAt is the timestamp of the last state snapshot written to disk.
	LastSnapshotAt time.Time `json:"last_snapshot_at"`

	// OfflineSince is the timestamp when offline mode started, nil if online.
	OfflineSince *time.Time `json:"offline_since,omitempty"`

	// ReconnectAttempts is the number of reconnection attempts since going offline.
	ReconnectAttempts int `json:"reconnect_attempts"`

	// CurrentPlan is the current Package_Plan name.
	CurrentPlan string `json:"current_plan"`
}

// FailureTracker tracks consecutive failures within a time window for state machine transitions.
type FailureTracker struct {
	mu sync.Mutex

	// ConsecutiveFailures is the number of consecutive failures recorded.
	ConsecutiveFailures int

	// FirstFailureAt is the timestamp of the first failure in the current sequence.
	FirstFailureAt time.Time

	// LastFailureAt is the timestamp of the most recent failure.
	LastFailureAt time.Time

	// WindowDuration is the time window for counting failures (e.g., 5 minutes for restart failures).
	WindowDuration time.Duration
}

// NewFailureTracker creates a new FailureTracker with the given window duration.
func NewFailureTracker(window time.Duration) *FailureTracker {
	return &FailureTracker{
		WindowDuration: window,
	}
}

// Record records a new failure. If the failure is outside the current window,
// the tracker resets and starts a new sequence.
func (ft *FailureTracker) Record() {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	now := time.Now()

	// If we have previous failures and the window has expired, reset
	if ft.ConsecutiveFailures > 0 && now.Sub(ft.FirstFailureAt) > ft.WindowDuration {
		ft.ConsecutiveFailures = 0
		ft.FirstFailureAt = time.Time{}
	}

	// Record the failure
	ft.ConsecutiveFailures++
	ft.LastFailureAt = now
	if ft.ConsecutiveFailures == 1 {
		ft.FirstFailureAt = now
	}
}

// ShouldEscalate returns true when the number of consecutive failures within
// the time window has reached or exceeded the given threshold.
func (ft *FailureTracker) ShouldEscalate(threshold int) bool {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	if ft.ConsecutiveFailures < threshold {
		return false
	}

	// Check if failures are within the window
	if ft.WindowDuration > 0 && time.Since(ft.FirstFailureAt) > ft.WindowDuration {
		// Window expired, reset
		ft.ConsecutiveFailures = 0
		ft.FirstFailureAt = time.Time{}
		return false
	}

	return true
}

// Reset resets the failure counter, typically called after a successful operation.
func (ft *FailureTracker) Reset() {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	ft.ConsecutiveFailures = 0
	ft.FirstFailureAt = time.Time{}
	ft.LastFailureAt = time.Time{}
}

// Count returns the current number of consecutive failures.
func (ft *FailureTracker) Count() int {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	return ft.ConsecutiveFailures
}
