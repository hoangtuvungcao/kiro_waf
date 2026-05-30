package monitor

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// HealthMonitor defines the interface for the health monitoring subsystem.
// It continuously checks XDP filter and binary health, and triggers self-recovery.
type HealthMonitor interface {
	// Start begins the health check loop in a separate goroutine.
	// It blocks until the context is cancelled or Stop is called.
	Start(ctx context.Context) error

	// Stop gracefully stops the health monitor.
	Stop() error

	// Status returns the current monitor status.
	Status() MonitorStatus
}

// healthMonitor is the concrete implementation of HealthMonitor.
type healthMonitor struct {
	config MonitorConfig
	client *http.Client

	// status holds the current monitor state, protected by mu.
	mu     sync.RWMutex
	status MonitorStatus

	// healthFailures tracks consecutive binary health check failures.
	healthFailures *FailureTracker

	// restartFailures tracks consecutive service restart failures.
	restartFailures *FailureTracker

	// xdpChecker handles XDP program health checking and reload.
	xdpChecker *XDPChecker

	// alertSender sends alerts to the Master Server.
	alertSender *AlertSender

	// serviceName is the systemd service name to restart (default: "kiro-client-waf").
	serviceName string

	// cancel is the cancel function for the monitor's context.
	cancel context.CancelFunc

	// done is closed when the monitor loop exits.
	done chan struct{}

	// running indicates whether the monitor is currently active.
	running bool

	// restartServiceFunc allows overriding the restart logic for testing.
	restartServiceFunc func(ctx context.Context) error
}

// MonitorOption is a functional option for configuring the health monitor.
type MonitorOption func(*healthMonitor)

// WithAlertSender sets the alert sender for the monitor.
func WithAlertSender(sender *AlertSender) MonitorOption {
	return func(m *healthMonitor) {
		m.alertSender = sender
	}
}

// WithXDPChecker sets the XDP checker for the monitor.
func WithXDPChecker(checker *XDPChecker) MonitorOption {
	return func(m *healthMonitor) {
		m.xdpChecker = checker
	}
}

// WithServiceName sets the systemd service name for restart operations.
func WithServiceName(name string) MonitorOption {
	return func(m *healthMonitor) {
		m.serviceName = name
	}
}

// WithRestartFunc overrides the service restart function (useful for testing).
func WithRestartFunc(fn func(ctx context.Context) error) MonitorOption {
	return func(m *healthMonitor) {
		m.restartServiceFunc = fn
	}
}

// New creates a new HealthMonitor with the given configuration.
// If config fields are zero, defaults are applied.
func New(config MonitorConfig, opts ...MonitorOption) HealthMonitor {
	cfg := applyDefaults(config)

	m := &healthMonitor{
		config: cfg,
		client: &http.Client{
			Timeout: cfg.HealthTimeout,
		},
		status: MonitorStatus{
			Mode:        ModeOnline,
			CurrentPlan: cfg.PlanConfig.PlanName,
		},
		healthFailures:  NewFailureTracker(cfg.CheckInterval * time.Duration(cfg.MaxConsecFailures+1)),
		restartFailures: NewFailureTracker(5 * time.Minute),
		serviceName:     "kiro-client-waf",
		done:            make(chan struct{}),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// applyDefaults fills in zero-value fields with defaults.
func applyDefaults(cfg MonitorConfig) MonitorConfig {
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = DefaultCheckInterval
	}
	if cfg.HealthEndpoint == "" {
		cfg.HealthEndpoint = DefaultHealthEndpoint
	}
	if cfg.HealthTimeout <= 0 {
		cfg.HealthTimeout = DefaultHealthTimeout
	}
	if cfg.MaxConsecFailures <= 0 {
		cfg.MaxConsecFailures = DefaultMaxConsecFailures
	}
	if cfg.MaxRestartFailures <= 0 {
		cfg.MaxRestartFailures = DefaultMaxRestartFailures
	}
	if cfg.RestartCooldown <= 0 {
		cfg.RestartCooldown = DefaultRestartCooldown
	}
	if cfg.OfflineThreshold <= 0 {
		cfg.OfflineThreshold = DefaultOfflineThreshold
	}
	if cfg.ReconnectInterval <= 0 {
		cfg.ReconnectInterval = DefaultReconnectInterval
	}
	if cfg.MaxReconnectBackoff <= 0 {
		cfg.MaxReconnectBackoff = DefaultMaxReconnectBackoff
	}
	if cfg.SnapshotInterval <= 0 {
		cfg.SnapshotInterval = DefaultSnapshotInterval
	}
	if cfg.SnapshotMaxSize <= 0 {
		cfg.SnapshotMaxSize = DefaultSnapshotMaxSize
	}
	if cfg.SnapshotPath == "" {
		cfg.SnapshotPath = DefaultSnapshotPath
	}
	if cfg.EmergencyDuration <= 0 {
		cfg.EmergencyDuration = DefaultEmergencyDuration
	}
	if cfg.EmergencyThreshold <= 0 {
		cfg.EmergencyThreshold = DefaultEmergencyThreshold
	}
	if cfg.DDoSThreshold <= 0 {
		cfg.DDoSThreshold = DefaultDDoSThreshold
	}
	if cfg.DDoSWindow <= 0 {
		cfg.DDoSWindow = DefaultDDoSWindow
	}
	return cfg
}

// Start begins the health check loop in a separate goroutine.
// Returns an error if the monitor is already running.
func (m *healthMonitor) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("monitor: already running")
	}
	m.running = true
	m.done = make(chan struct{})
	m.mu.Unlock()

	// Create a child context that can be cancelled by Stop()
	monCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	log.Printf("Health monitor started: interval=%s endpoint=%s timeout=%s",
		m.config.CheckInterval, m.config.HealthEndpoint, m.config.HealthTimeout)

	// Run the health check loop in a separate goroutine
	go m.run(monCtx)

	return nil
}

// Stop gracefully stops the health monitor.
func (m *healthMonitor) Stop() error {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return fmt.Errorf("monitor: not running")
	}
	m.mu.Unlock()

	// Cancel the monitor context
	if m.cancel != nil {
		m.cancel()
	}

	// Wait for the loop to exit
	<-m.done

	m.mu.Lock()
	m.running = false
	m.mu.Unlock()

	log.Printf("Health monitor stopped")
	return nil
}

// Status returns a copy of the current monitor status.
func (m *healthMonitor) Status() MonitorStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// run is the main health check loop that runs in its own goroutine.
// It performs health checks every CheckInterval (default 10 seconds).
func (m *healthMonitor) run(ctx context.Context) {
	defer close(m.done)

	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.performHealthCheck(ctx)
			m.performXDPCheck(ctx)
		}
	}
}

// performHealthCheck executes a single health check cycle.
// It checks the binary health endpoint and updates the monitor status.
func (m *healthMonitor) performHealthCheck(ctx context.Context) {
	healthy := m.checkBinaryHealth(ctx)

	m.mu.Lock()
	m.status.LastCheckAt = time.Now()
	m.status.BinaryHealthy = healthy

	if healthy {
		m.status.ConsecFailures = 0
		m.mu.Unlock()
		m.healthFailures.Reset()
		return
	}

	// Health check failed
	m.healthFailures.Record()
	m.status.ConsecFailures = m.healthFailures.Count()
	consecFailures := m.status.ConsecFailures
	m.mu.Unlock()

	log.Printf("Health monitor: binary health check failed (consecutive: %d/%d)",
		consecFailures, m.config.MaxConsecFailures)

	// Check if we should trigger a restart
	if consecFailures >= m.config.MaxConsecFailures {
		m.handleRestartNeeded(ctx)
	}
}

// checkBinaryHealth calls the health endpoint and returns true if it responds HTTP 200.
func (m *healthMonitor) checkBinaryHealth(ctx context.Context) bool {
	url := fmt.Sprintf("http://127.0.0.1%s", m.config.HealthEndpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Printf("Health monitor: create request failed: %v", err)
		return false
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// handleRestartNeeded is called when consecutive health check failures reach the threshold.
// It attempts to restart the service and tracks restart failures.
// Escalation logic: 3 restart failures within 5 minutes → alert Master + wait 60 seconds.
func (m *healthMonitor) handleRestartNeeded(ctx context.Context) {
	log.Printf("Health monitor: max consecutive failures reached (%d), attempting service restart",
		m.config.MaxConsecFailures)

	// Check if we've exceeded restart failure threshold (3 failures in 5 minutes)
	if m.restartFailures.ShouldEscalate(m.config.MaxRestartFailures) {
		log.Printf("CRITICAL: Health monitor: %d restart failures within 5 minutes, alerting Master and waiting %s",
			m.config.MaxRestartFailures, m.config.RestartCooldown)

		// Alert Master Server about restart failures
		m.sendAlert("restart_failed", fmt.Sprintf(
			"Service restart failed %d consecutive times within 5 minutes on node",
			m.config.MaxRestartFailures))

		// Wait for cooldown period (60 seconds)
		select {
		case <-ctx.Done():
			return
		case <-time.After(m.config.RestartCooldown):
		}

		// Reset restart failures after cooldown
		m.restartFailures.Reset()
	}

	// Attempt restart via systemctl
	err := m.restartService(ctx)
	if err != nil {
		m.restartFailures.Record()
		log.Printf("Health monitor: service restart failed: %v (restart failures: %d/%d)",
			err, m.restartFailures.Count(), m.config.MaxRestartFailures)
		return
	}

	// Restart succeeded
	m.restartFailures.Reset()
	m.healthFailures.Reset()

	m.mu.Lock()
	m.status.ConsecFailures = 0
	m.status.BinaryHealthy = true
	m.mu.Unlock()

	log.Printf("Health monitor: service restart succeeded")
}

// restartService attempts to restart the client service via systemctl.
func (m *healthMonitor) restartService(ctx context.Context) error {
	// Allow override for testing
	if m.restartServiceFunc != nil {
		return m.restartServiceFunc(ctx)
	}

	log.Printf("Health monitor: restarting %s service via systemctl", m.serviceName)

	cmd := exec.CommandContext(ctx, "systemctl", "restart", m.serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl restart %s: %s: %w",
			m.serviceName, strings.TrimSpace(string(output)), err)
	}

	// Verify the service is active after restart
	verifyCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Wait a moment for the service to start
	select {
	case <-verifyCtx.Done():
		return fmt.Errorf("context cancelled while verifying restart")
	case <-time.After(2 * time.Second):
	}

	active, err := m.checkServiceActive(verifyCtx)
	if err != nil {
		return fmt.Errorf("failed to verify service status after restart: %w", err)
	}
	if !active {
		return fmt.Errorf("service %s not active after restart", m.serviceName)
	}

	return nil
}

// checkServiceActive checks if the systemd service is in active state.
func (m *healthMonitor) checkServiceActive(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, "systemctl", "is-active", m.serviceName)
	output, err := cmd.Output()
	if err != nil {
		// systemctl is-active returns non-zero when service is not active
		return false, nil
	}

	status := strings.TrimSpace(string(output))
	return status == "active", nil
}

// performXDPCheck checks the XDP program status and handles detachment.
func (m *healthMonitor) performXDPCheck(ctx context.Context) {
	if m.xdpChecker == nil {
		return
	}

	xdpStatus := m.xdpChecker.CheckXDP(ctx)

	m.mu.Lock()
	m.status.XDPAttached = xdpStatus.Attached
	m.mu.Unlock()

	if !xdpStatus.Attached {
		log.Printf("Health monitor: XDP program not attached to interface")
		if err := m.xdpChecker.HandleDetached(ctx); err != nil {
			log.Printf("Health monitor: XDP reattach failed: %v", err)
		} else {
			m.mu.Lock()
			m.status.XDPAttached = true
			m.mu.Unlock()
		}
	}
}

// sendAlert sends an alert to the Master Server if an alert sender is configured.
func (m *healthMonitor) sendAlert(alertType string, message string) {
	if m.alertSender != nil {
		m.alertSender.SendAlert(alertType, message)
	} else {
		log.Printf("Health monitor: ALERT [%s]: %s (no alert sender configured)", alertType, message)
	}
}
