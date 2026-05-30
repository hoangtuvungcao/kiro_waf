// Package updater implements the OTA (Over-The-Air) automatic update system
// for Kiro WAF Client Node. It polls the Master Server for available updates,
// downloads and verifies binaries, performs atomic replacement, and supports
// rollback on health check failure.
package updater

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultPollInterval is the default interval between update checks.
	DefaultPollInterval = 300 * time.Second

	// MinPollInterval is the minimum allowed poll interval.
	MinPollInterval = 60 * time.Second

	// MaxPollInterval is the maximum allowed poll interval.
	MaxPollInterval = 86400 * time.Second

	// DefaultDownloadTimeout is the default timeout for downloading binaries.
	DefaultDownloadTimeout = 5 * time.Minute

	// DefaultHealthTimeout is the default timeout for health checks after update.
	DefaultHealthTimeout = 30 * time.Second
)

// Updater defines the interface for OTA update operations.
type Updater interface {
	// CheckForUpdate polls master server for available updates.
	CheckForUpdate(ctx context.Context) (*UpdateInfo, error)

	// DownloadAndVerify downloads binary and verifies SHA-256.
	DownloadAndVerify(ctx context.Context, info *UpdateInfo) (string, error)

	// ApplyUpdate performs atomic binary replacement.
	ApplyUpdate(ctx context.Context, newBinaryPath string) error

	// Rollback restores previous binary version.
	Rollback(ctx context.Context) error
}

// UpdaterConfig holds configuration for the OTA updater.
type UpdaterConfig struct {
	// MasterURL is the URL of the Master Server (e.g., "https://firewall.vpsgen.com").
	MasterURL string

	// LicenseKey is the license key for authentication.
	LicenseKey string

	// Component is the component name to check updates for (e.g., "kiro-client-waf").
	Component string

	// Channel is the update channel (e.g., "stable", "beta").
	Channel string

	// CurrentVersion is the currently running version.
	CurrentVersion string

	// PollInterval is the interval between update checks.
	// Default 300s, min 60s, max 86400s. Values outside range are clamped.
	PollInterval time.Duration

	// DownloadTimeout is the timeout for downloading binaries. Default 5 minutes.
	DownloadTimeout time.Duration

	// HealthTimeout is the timeout for health checks after update. Default 30 seconds.
	HealthTimeout time.Duration

	// BinaryPath is the path to the running binary (e.g., "/usr/local/bin/kiro-client-waf").
	BinaryPath string

	// BackupPath is the path for storing the previous binary version.
	BackupPath string
}

// UpdateInfo contains information about an available update.
type UpdateInfo struct {
	// Version is the new version string.
	Version string `json:"version"`

	// ArtifactURL is the URL to download the binary from.
	ArtifactURL string `json:"artifact_url"`

	// SHA256 is the expected SHA-256 checksum of the binary.
	SHA256 string `json:"sha256"`

	// Notes contains release notes.
	Notes string `json:"notes"`
}

// UpdateState tracks the current state of the OTA updater.
type UpdateState struct {
	// CurrentVersion is the version currently running.
	CurrentVersion string `json:"current_version"`

	// PreviousVersion is the version before the last update.
	PreviousVersion string `json:"previous_version"`

	// LastCheckAt is the timestamp of the last update check.
	LastCheckAt time.Time `json:"last_check_at"`

	// LastUpdateAt is the timestamp of the last successful update.
	LastUpdateAt time.Time `json:"last_update_at"`

	// BackupPath is the path to the backup binary.
	BackupPath string `json:"backup_path"`

	// Status is the current updater status: idle, downloading, applying, rolling_back.
	Status string `json:"status"`
}

// ClampPollInterval clamps the given duration to the valid range [MinPollInterval, MaxPollInterval].
// If d is zero or negative, DefaultPollInterval is returned.
func ClampPollInterval(d time.Duration) time.Duration {
	if d <= 0 {
		return DefaultPollInterval
	}
	if d < MinPollInterval {
		return MinPollInterval
	}
	if d > MaxPollInterval {
		return MaxPollInterval
	}
	return d
}

// OTAUpdater implements the Updater interface for automatic binary updates.
type OTAUpdater struct {
	config UpdaterConfig
	state  UpdateState
	client *http.Client

	// mu protects state access.
	mu sync.RWMutex

	// pushCh receives signals from heartbeat responses indicating an update is available.
	pushCh chan struct{}
}

// NewOTAUpdater creates a new OTAUpdater with the given configuration.
// Poll interval is automatically clamped to valid range.
func NewOTAUpdater(config UpdaterConfig) *OTAUpdater {
	config.PollInterval = ClampPollInterval(config.PollInterval)

	if config.DownloadTimeout <= 0 {
		config.DownloadTimeout = DefaultDownloadTimeout
	}
	if config.HealthTimeout <= 0 {
		config.HealthTimeout = DefaultHealthTimeout
	}

	return &OTAUpdater{
		config: config,
		state: UpdateState{
			CurrentVersion: config.CurrentVersion,
			BackupPath:     config.BackupPath,
			Status:         "idle",
		},
		client: &http.Client{Timeout: 30 * time.Second},
		pushCh: make(chan struct{}, 1),
	}
}

// updateCheckRequest is the payload sent to Master Server for update checks.
type updateCheckRequest struct {
	Component      string `json:"component"`
	Channel        string `json:"channel"`
	CurrentVersion string `json:"current_version"`
}

// updateCheckResponse is the response from Master Server for update checks.
type updateCheckResponse struct {
	UpdateAvailable bool        `json:"update_available"`
	Release         *UpdateInfo `json:"release"`
}

// CheckForUpdate polls the master server for available updates.
// Returns UpdateInfo if an update is available, nil if current version is up to date.
func (u *OTAUpdater) CheckForUpdate(ctx context.Context) (*UpdateInfo, error) {
	endpoint := strings.TrimRight(u.config.MasterURL, "/") + "/api/v1/update/check"

	payload := updateCheckRequest{
		Component:      u.config.Component,
		Channel:        u.config.Channel,
		CurrentVersion: u.config.CurrentVersion,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("updater: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("updater: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-License-Key", u.config.LicenseKey)

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("updater: http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("updater: server returned status %d", resp.StatusCode)
	}

	var result updateCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("updater: decode response: %w", err)
	}

	// Update last check timestamp
	u.mu.Lock()
	u.state.LastCheckAt = time.Now()
	u.mu.Unlock()

	if !result.UpdateAvailable || result.Release == nil {
		return nil, nil
	}

	return result.Release, nil
}

// DownloadAndVerify downloads the binary and verifies its SHA-256 checksum.
// It uses config.DownloadTimeout as the context deadline, writes to a temp file,
// computes SHA-256, and compares with info.SHA256. On mismatch or network failure,
// partial downloads are cleaned up and an error is returned.
func (u *OTAUpdater) DownloadAndVerify(ctx context.Context, info *UpdateInfo) (string, error) {
	if info == nil {
		return "", fmt.Errorf("updater: nil update info")
	}
	if info.ArtifactURL == "" {
		return "", fmt.Errorf("updater: empty artifact URL")
	}
	if info.SHA256 == "" {
		return "", fmt.Errorf("updater: empty expected SHA-256")
	}

	// Set status to downloading
	u.mu.Lock()
	u.state.Status = "downloading"
	u.mu.Unlock()

	defer func() {
		u.mu.Lock()
		if u.state.Status == "downloading" {
			u.state.Status = "idle"
		}
		u.mu.Unlock()
	}()

	// Create download context with timeout
	downloadCtx, cancel := context.WithTimeout(ctx, u.config.DownloadTimeout)
	defer cancel()

	// Create HTTP request with license key header
	req, err := http.NewRequestWithContext(downloadCtx, http.MethodGet, info.ArtifactURL, nil)
	if err != nil {
		return "", fmt.Errorf("updater: create download request: %w", err)
	}
	req.Header.Set("X-License-Key", u.config.LicenseKey)

	// Execute download
	resp, err := u.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("updater: download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("updater: download server returned status %d", resp.StatusCode)
	}

	// Create temp file for download
	tmpFile, err := os.CreateTemp("", "kiro-update-*.bin")
	if err != nil {
		return "", fmt.Errorf("updater: create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Cleanup on any error
	success := false
	defer func() {
		if !success {
			tmpFile.Close()
			os.Remove(tmpPath)
		}
	}()

	// Download and compute SHA-256 simultaneously
	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)

	if _, err := io.Copy(writer, resp.Body); err != nil {
		return "", fmt.Errorf("updater: download write failed: %w", err)
	}

	// Close the file to flush writes
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("updater: close temp file: %w", err)
	}

	// Verify SHA-256 checksum
	computedHash := hex.EncodeToString(hasher.Sum(nil))
	expectedHash := strings.ToLower(strings.TrimSpace(info.SHA256))

	if computedHash != expectedHash {
		return "", fmt.Errorf("updater: SHA-256 mismatch: expected %s, got %s", expectedHash, computedHash)
	}

	log.Printf("OTA updater: download verified: version=%s sha256=%s path=%s",
		info.Version, computedHash, tmpPath)

	success = true
	return tmpPath, nil
}

// ApplyUpdate performs atomic binary replacement using rename(2).
// It backs up the current binary to config.BackupPath, then atomically
// replaces the binary at config.BinaryPath with the new binary.
// The new binary is given 755 permissions before replacement.
func (u *OTAUpdater) ApplyUpdate(ctx context.Context, newBinaryPath string) error {
	if newBinaryPath == "" {
		return fmt.Errorf("updater: empty new binary path")
	}
	if u.config.BinaryPath == "" {
		return fmt.Errorf("updater: empty target binary path in config")
	}
	if u.config.BackupPath == "" {
		return fmt.Errorf("updater: empty backup path in config")
	}

	// Set status to applying
	u.mu.Lock()
	u.state.Status = "applying"
	u.mu.Unlock()

	defer func() {
		u.mu.Lock()
		if u.state.Status == "applying" {
			u.state.Status = "idle"
		}
		u.mu.Unlock()
	}()

	// Set correct permissions on new binary (755)
	if err := os.Chmod(newBinaryPath, 0755); err != nil {
		return fmt.Errorf("updater: chmod new binary: %w", err)
	}

	// Backup current binary to backup path (overwrite any existing backup)
	// This ensures exactly one previous version is retained
	if err := copyFile(u.config.BinaryPath, u.config.BackupPath); err != nil {
		// If current binary doesn't exist, that's okay for fresh installs
		if !os.IsNotExist(err) {
			return fmt.Errorf("updater: backup current binary: %w", err)
		}
		log.Printf("OTA updater: no existing binary to backup at %s", u.config.BinaryPath)
	}

	// Atomic replacement using rename(2)
	if err := os.Rename(newBinaryPath, u.config.BinaryPath); err != nil {
		return fmt.Errorf("updater: atomic rename failed: %w", err)
	}

	// Update state
	u.mu.Lock()
	u.state.PreviousVersion = u.state.CurrentVersion
	u.state.LastUpdateAt = time.Now()
	u.state.Status = "idle"
	u.mu.Unlock()

	log.Printf("OTA updater: binary replaced atomically: path=%s backup=%s",
		u.config.BinaryPath, u.config.BackupPath)

	return nil
}

// copyFile copies a file from src to dst, creating or overwriting dst.
// It preserves the file permissions of the source.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return dstFile.Close()
}

// Rollback restores the previous binary version from backup.
// It checks if a backup exists at config.BackupPath, then atomically
// renames the backup to config.BinaryPath using rename(2).
func (u *OTAUpdater) Rollback(ctx context.Context) error {
	if u.config.BackupPath == "" {
		return fmt.Errorf("updater: empty backup path in config")
	}
	if u.config.BinaryPath == "" {
		return fmt.Errorf("updater: empty binary path in config")
	}

	// Set status to rolling_back
	u.mu.Lock()
	u.state.Status = "rolling_back"
	u.mu.Unlock()

	log.Printf("OTA updater: rollback initiated: backup=%s target=%s",
		u.config.BackupPath, u.config.BinaryPath)

	// Check if backup exists
	if _, err := os.Stat(u.config.BackupPath); err != nil {
		u.mu.Lock()
		u.state.Status = "idle"
		u.mu.Unlock()
		if os.IsNotExist(err) {
			return fmt.Errorf("updater: backup not found at %s", u.config.BackupPath)
		}
		return fmt.Errorf("updater: cannot access backup: %w", err)
	}

	// Atomic replacement: rename backup to binary path
	if err := os.Rename(u.config.BackupPath, u.config.BinaryPath); err != nil {
		u.mu.Lock()
		u.state.Status = "idle"
		u.mu.Unlock()
		return fmt.Errorf("updater: rollback rename failed: %w", err)
	}

	// Update state
	u.mu.Lock()
	u.state.CurrentVersion = u.state.PreviousVersion
	u.state.PreviousVersion = ""
	u.state.Status = "idle"
	u.mu.Unlock()

	log.Printf("OTA updater: rollback completed: restored binary at %s", u.config.BinaryPath)
	return nil
}

// ServiceChecker defines the interface for checking systemd service health.
// This allows dependency injection for testing.
type ServiceChecker interface {
	// IsActive returns true if the service is in active (running) state.
	IsActive(ctx context.Context) (bool, error)

	// Restart restarts the service.
	Restart(ctx context.Context) error
}

// SystemdServiceChecker checks service health via systemd (systemctl).
type SystemdServiceChecker struct {
	ServiceName string
}

// IsActive checks if the systemd service is active using systemctl is-active.
func (s *SystemdServiceChecker) IsActive(ctx context.Context) (bool, error) {
	return checkSystemdActive(ctx, s.ServiceName)
}

// Restart restarts the systemd service using systemctl restart.
func (s *SystemdServiceChecker) Restart(ctx context.Context) error {
	return restartSystemdService(ctx, s.ServiceName)
}

// HealthCheckAndRollback performs a health check after applying an update.
// It waits for the service to become active within config.HealthTimeout (default 30s).
// If the service does not become active in time, it automatically rolls back
// to the previous binary and restarts the service.
func (u *OTAUpdater) HealthCheckAndRollback(ctx context.Context, checker ServiceChecker) error {
	timeout := u.config.HealthTimeout
	if timeout <= 0 {
		timeout = DefaultHealthTimeout
	}

	log.Printf("OTA updater: health check started: timeout=%s", timeout)

	deadline := time.Now().Add(timeout)
	pollInterval := 2 * time.Second

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return fmt.Errorf("updater: health check cancelled: %w", ctx.Err())
		default:
		}

		active, err := checker.IsActive(ctx)
		if err != nil {
			log.Printf("OTA updater: health check probe error: %v", err)
		} else if active {
			log.Printf("OTA updater: health check passed: service is active")
			return nil
		}

		// Wait before next probe
		select {
		case <-ctx.Done():
			return fmt.Errorf("updater: health check cancelled: %w", ctx.Err())
		case <-time.After(pollInterval):
		}
	}

	// Health check failed — auto-rollback
	log.Printf("OTA updater: health check failed: service not active within %s, initiating rollback", timeout)

	if err := u.Rollback(ctx); err != nil {
		return fmt.Errorf("updater: health check failed and rollback failed: %w", err)
	}

	// Restart service with old binary
	log.Printf("OTA updater: restarting service with rolled-back binary")
	if err := checker.Restart(ctx); err != nil {
		return fmt.Errorf("updater: rollback succeeded but service restart failed: %w", err)
	}

	log.Printf("OTA updater: rollback and restart completed successfully")
	return fmt.Errorf("updater: health check failed, rolled back to previous version")
}

// TriggerUpdateCheck sends a signal to trigger an immediate update check.
// This is called when the heartbeat response indicates an update is available.
// Non-blocking: if a trigger is already pending, the new signal is dropped.
func (u *OTAUpdater) TriggerUpdateCheck() {
	select {
	case u.pushCh <- struct{}{}:
	default:
		// Already a pending trigger, skip
	}
}

// State returns a copy of the current update state.
func (u *OTAUpdater) State() UpdateState {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.state
}

// Config returns the updater configuration.
func (u *OTAUpdater) Config() UpdaterConfig {
	return u.config
}

// Run starts the OTA updater loop. It polls for updates at the configured interval
// and also responds to push triggers from heartbeat responses.
// Run blocks until the context is cancelled.
func (u *OTAUpdater) Run(ctx context.Context) {
	interval := u.config.PollInterval
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("OTA updater started: component=%s channel=%s version=%s interval=%s",
		u.config.Component, u.config.Channel, u.config.CurrentVersion, interval)

	for {
		select {
		case <-ctx.Done():
			log.Printf("OTA updater stopped")
			return
		case <-ticker.C:
			u.performUpdateCheck(ctx)
		case <-u.pushCh:
			log.Printf("OTA updater: heartbeat push trigger received, checking immediately")
			u.performUpdateCheck(ctx)
			// Reset ticker after push trigger to avoid double-check
			ticker.Reset(interval)
		}
	}
}

// performUpdateCheck executes a single update check cycle.
func (u *OTAUpdater) performUpdateCheck(ctx context.Context) {
	info, err := u.CheckForUpdate(ctx)
	if err != nil {
		log.Printf("OTA updater: check failed: %v", err)
		return
	}

	if info == nil {
		return
	}

	log.Printf("OTA updater: update available: version=%s artifact_url=%s",
		info.Version, info.ArtifactURL)
}
