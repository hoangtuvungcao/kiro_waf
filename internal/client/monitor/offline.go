package monitor

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// OfflineManager handles offline mode transitions, exponential backoff reconnection,
// and configuration sync when connectivity is restored.
// Requirements: 13.5, 13.6, 13.7
type OfflineManager struct {
	mu sync.RWMutex

	// config holds the monitor configuration for offline thresholds and backoff.
	config MonitorConfig

	// alertSender sends alerts/reports to the Master Server.
	alertSender *AlertSender

	// state tracks the current offline state.
	state OfflineState

	// lastMasterContact is the timestamp of the last successful Master Server communication.
	lastMasterContact time.Time

	// reconnectAttempts counts the number of reconnection attempts since going offline.
	reconnectAttempts int

	// currentBackoff is the current backoff duration for reconnection attempts.
	currentBackoff time.Duration

	// checkMasterFunc is the function used to check Master Server connectivity.
	// Can be overridden for testing.
	checkMasterFunc func(ctx context.Context) error

	// syncConfigFunc is called when connectivity is restored to sync configuration.
	// Can be overridden for testing.
	syncConfigFunc func(ctx context.Context) error

	// onModeChange is called when the operation mode changes.
	onModeChange func(mode OperationMode)

	// nowFunc returns the current time (overridable for testing).
	nowFunc func() time.Time
}

// OfflineState represents the current state of the offline manager.
type OfflineState struct {
	// IsOffline indicates whether the client is currently in offline mode.
	IsOffline bool

	// OfflineSince is when offline mode started.
	OfflineSince time.Time

	// LastReconnectAttempt is the timestamp of the last reconnection attempt.
	LastReconnectAttempt time.Time

	// ReconnectAttempts is the total number of reconnection attempts.
	ReconnectAttempts int

	// CurrentBackoff is the current backoff interval.
	CurrentBackoff time.Duration
}

// OfflineReport is sent to the Master Server when connectivity is restored.
type OfflineReport struct {
	// OfflineStart is when the offline period began.
	OfflineStart time.Time `json:"offline_start"`

	// OfflineEnd is when connectivity was restored.
	OfflineEnd time.Time `json:"offline_end"`

	// ReconnectAttempts is the total number of reconnection attempts during the offline period.
	ReconnectAttempts int `json:"reconnect_attempts"`

	// Duration is the total offline duration.
	Duration time.Duration `json:"duration"`
}

// OfflineManagerOption is a functional option for configuring the OfflineManager.
type OfflineManagerOption func(*OfflineManager)

// WithCheckMasterFunc sets the function used to check Master Server connectivity.
func WithCheckMasterFunc(fn func(ctx context.Context) error) OfflineManagerOption {
	return func(om *OfflineManager) {
		om.checkMasterFunc = fn
	}
}

// WithSyncConfigFunc sets the function called when connectivity is restored.
func WithSyncConfigFunc(fn func(ctx context.Context) error) OfflineManagerOption {
	return func(om *OfflineManager) {
		om.syncConfigFunc = fn
	}
}

// WithOnModeChange sets the callback for mode changes.
func WithOnModeChange(fn func(mode OperationMode)) OfflineManagerOption {
	return func(om *OfflineManager) {
		om.onModeChange = fn
	}
}

// WithNowFunc sets the time function (for testing).
func WithNowFunc(fn func() time.Time) OfflineManagerOption {
	return func(om *OfflineManager) {
		om.nowFunc = fn
	}
}

// NewOfflineManager creates a new OfflineManager with the given configuration.
func NewOfflineManager(config MonitorConfig, alertSender *AlertSender, opts ...OfflineManagerOption) *OfflineManager {
	om := &OfflineManager{
		config:         config,
		alertSender:    alertSender,
		currentBackoff: config.ReconnectInterval,
		nowFunc:        time.Now,
	}

	for _, opt := range opts {
		opt(om)
	}

	// Initialize lastMasterContact using nowFunc (which may be overridden by opts)
	om.lastMasterContact = om.nowFunc()

	return om
}

// RecordMasterContact records a successful communication with the Master Server.
// This resets the offline timer and transitions back to online if currently offline.
func (om *OfflineManager) RecordMasterContact(ctx context.Context) {
	om.mu.Lock()
	wasOffline := om.state.IsOffline
	offlineSince := om.state.OfflineSince
	attempts := om.state.ReconnectAttempts
	om.lastMasterContact = om.nowFunc()

	if wasOffline {
		// Transition from offline to online
		om.state.IsOffline = false
		om.state.ReconnectAttempts = 0
		om.currentBackoff = om.config.ReconnectInterval
		om.reconnectAttempts = 0
	}
	om.mu.Unlock()

	if wasOffline {
		log.Printf("Health monitor: connectivity restored after offline period (since %s, %d reconnect attempts)",
			offlineSince.Format(time.RFC3339), attempts)

		// Sync configuration from Master
		om.syncConfig(ctx)

		// Send offline report
		om.sendOfflineReport(offlineSince, om.nowFunc(), attempts)

		// Notify mode change
		if om.onModeChange != nil {
			om.onModeChange(ModeOnline)
		}
	}
}

// CheckOfflineStatus checks if the client should transition to offline mode.
// Returns true if the client is currently in offline mode.
func (om *OfflineManager) CheckOfflineStatus() bool {
	om.mu.RLock()
	defer om.mu.RUnlock()
	return om.state.IsOffline
}

// GetState returns a copy of the current offline state.
func (om *OfflineManager) GetState() OfflineState {
	om.mu.RLock()
	defer om.mu.RUnlock()
	return om.state
}

// EvaluateConnectivity evaluates the current connectivity status and transitions
// to offline mode if the Master Server has been unreachable for longer than OfflineThreshold.
// Returns true if the client transitioned to offline mode during this call.
func (om *OfflineManager) EvaluateConnectivity() bool {
	om.mu.Lock()
	defer om.mu.Unlock()

	now := om.nowFunc()

	// Already offline, no transition needed
	if om.state.IsOffline {
		return false
	}

	// Check if we've exceeded the offline threshold
	elapsed := now.Sub(om.lastMasterContact)
	if elapsed <= om.config.OfflineThreshold {
		return false
	}

	// Transition to offline mode
	om.state.IsOffline = true
	om.state.OfflineSince = now
	om.state.ReconnectAttempts = 0
	om.currentBackoff = om.config.ReconnectInterval

	log.Printf("Health monitor: switching to offline mode (no Master contact for %s, threshold %s)",
		elapsed.Round(time.Second), om.config.OfflineThreshold)

	// Notify mode change
	if om.onModeChange != nil {
		om.onModeChange(ModeOffline)
	}

	return true
}

// AttemptReconnect attempts to reconnect to the Master Server using exponential backoff.
// Returns true if reconnection was successful.
// Backoff: starts at ReconnectInterval (30s), doubles each attempt, max MaxReconnectBackoff (5min).
func (om *OfflineManager) AttemptReconnect(ctx context.Context) bool {
	om.mu.RLock()
	if !om.state.IsOffline {
		om.mu.RUnlock()
		return true // Already online
	}

	lastAttempt := om.state.LastReconnectAttempt
	currentBackoff := om.currentBackoff
	om.mu.RUnlock()

	now := om.nowFunc()

	// Check if enough time has passed since the last attempt (respect backoff)
	if !lastAttempt.IsZero() && now.Sub(lastAttempt) < currentBackoff {
		return false // Not time to retry yet
	}

	// Attempt reconnection
	om.mu.Lock()
	om.state.LastReconnectAttempt = now
	om.state.ReconnectAttempts++
	om.reconnectAttempts = om.state.ReconnectAttempts
	om.mu.Unlock()

	log.Printf("Health monitor: attempting reconnection to Master (attempt %d, backoff %s)",
		om.reconnectAttempts, currentBackoff.Round(time.Second))

	var err error
	if om.checkMasterFunc != nil {
		err = om.checkMasterFunc(ctx)
	} else {
		err = fmt.Errorf("no check master function configured")
	}

	if err != nil {
		// Reconnection failed, increase backoff
		om.mu.Lock()
		om.currentBackoff = om.nextBackoff(om.currentBackoff)
		om.state.CurrentBackoff = om.currentBackoff
		om.mu.Unlock()

		log.Printf("Health monitor: reconnection failed: %v (next backoff: %s)",
			err, om.currentBackoff.Round(time.Second))
		return false
	}

	// Reconnection succeeded
	om.RecordMasterContact(ctx)
	return true
}

// nextBackoff calculates the next backoff duration using exponential backoff.
// Formula: min(current * 2, MaxReconnectBackoff)
// Initial: ReconnectInterval (30s), doubles each time, max MaxReconnectBackoff (5min).
func (om *OfflineManager) nextBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > om.config.MaxReconnectBackoff {
		next = om.config.MaxReconnectBackoff
	}
	return next
}

// CalculateBackoff calculates the backoff duration for a given attempt number.
// Formula: min(ReconnectInterval * 2^(attempt-1), MaxReconnectBackoff)
// This is a pure function useful for testing.
func CalculateBackoff(baseInterval, maxBackoff time.Duration, attempt int) time.Duration {
	if attempt <= 0 {
		return baseInterval
	}

	backoff := baseInterval
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff > maxBackoff {
			return maxBackoff
		}
	}
	return backoff
}

// syncConfig calls the sync configuration function when connectivity is restored.
func (om *OfflineManager) syncConfig(ctx context.Context) {
	if om.syncConfigFunc == nil {
		log.Printf("Health monitor: no sync config function configured, skipping config sync")
		return
	}

	log.Printf("Health monitor: syncing configuration from Master Server")
	if err := om.syncConfigFunc(ctx); err != nil {
		log.Printf("Health monitor: config sync failed: %v", err)
	} else {
		log.Printf("Health monitor: configuration synced successfully")
	}
}

// sendOfflineReport sends a report about the offline period to the Master Server.
func (om *OfflineManager) sendOfflineReport(start, end time.Time, attempts int) {
	report := OfflineReport{
		OfflineStart:      start,
		OfflineEnd:        end,
		ReconnectAttempts: attempts,
		Duration:          end.Sub(start),
	}

	if om.alertSender != nil {
		message := fmt.Sprintf("Offline period: %s to %s (%s), %d reconnect attempts",
			report.OfflineStart.Format(time.RFC3339),
			report.OfflineEnd.Format(time.RFC3339),
			report.Duration.Round(time.Second),
			report.ReconnectAttempts)
		om.alertSender.SendAlert("offline_report", message)
	} else {
		log.Printf("Health monitor: offline report (no alert sender): start=%s end=%s attempts=%d",
			report.OfflineStart.Format(time.RFC3339),
			report.OfflineEnd.Format(time.RFC3339),
			report.ReconnectAttempts)
	}
}

// TimeSinceLastContact returns the duration since the last successful Master Server contact.
func (om *OfflineManager) TimeSinceLastContact() time.Duration {
	om.mu.RLock()
	defer om.mu.RUnlock()
	return om.nowFunc().Sub(om.lastMasterContact)
}
