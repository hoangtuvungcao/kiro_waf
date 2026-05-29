// Package update triển khai cơ chế cập nhật binary qua CLI.
// Bao gồm: check, apply (download + SHA-256 verify + atomic replace), rollback.
package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// updateCheckRequest is the payload sent to POST /api/v1/update/check.
type updateCheckRequest struct {
	Component      string `json:"component"`
	Channel        string `json:"channel"`
	CurrentVersion string `json:"current_version"`
}

// updateCheckResponse is the response from POST /api/v1/update/check.
type updateCheckResponse struct {
	UpdateAvailable bool         `json:"update_available"`
	Release         *releaseInfo `json:"release,omitempty"`
}

// releaseInfo contains metadata about an available release.
type releaseInfo struct {
	Version     string `json:"version"`
	ArtifactURL string `json:"artifact_url"`
	SHA256      string `json:"sha256"`
	Notes       string `json:"notes"`
}

// Check calls the master server's update check API and displays information
// about available updates.
func Check(masterURL, component, channel, currentVersion string) error {
	if masterURL == "" {
		return errors.New("master URL is required")
	}
	if component == "" {
		return errors.New("component is required")
	}
	if channel == "" {
		channel = "stable"
	}

	endpoint := strings.TrimRight(masterURL, "/") + "/api/v1/update/check"

	payload := updateCheckRequest{
		Component:      component,
		Channel:        channel,
		CurrentVersion: currentVersion,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("update check request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update check returned HTTP %d", resp.StatusCode)
	}

	var result updateCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if !result.UpdateAvailable {
		fmt.Printf("[%s] Already up to date (version: %s, channel: %s)\n", component, currentVersion, channel)
		return nil
	}

	fmt.Printf("[%s] Update available!\n", component)
	fmt.Printf("  Current version: %s\n", currentVersion)
	fmt.Printf("  New version:     %s\n", result.Release.Version)
	fmt.Printf("  Artifact URL:    %s\n", result.Release.ArtifactURL)
	fmt.Printf("  SHA-256:         %s\n", result.Release.SHA256)
	if result.Release.Notes != "" {
		fmt.Printf("  Notes:           %s\n", result.Release.Notes)
	}
	fmt.Println()
	fmt.Println("Run 'kiro-cli update apply' to install the update.")

	return nil
}

// Apply downloads the artifact, verifies SHA-256, performs atomic binary replace,
// restarts the service, and runs a health check within 30 seconds.
// If SHA-256 mismatches, it aborts and logs the error.
// If health check fails after replacement, it automatically rolls back.
func Apply(masterURL, component, channel, currentVersion, binaryPath, serviceName string) error {
	if masterURL == "" {
		return errors.New("master URL is required")
	}
	if component == "" {
		return errors.New("component is required")
	}
	if channel == "" {
		channel = "stable"
	}
	if binaryPath == "" {
		return errors.New("binary path is required")
	}
	if serviceName == "" {
		return errors.New("service name is required")
	}

	// Step 1: Check for updates.
	log.Printf("[update] Checking for updates: component=%s channel=%s current=%s", component, channel, currentVersion)

	endpoint := strings.TrimRight(masterURL, "/") + "/api/v1/update/check"
	payload := updateCheckRequest{
		Component:      component,
		Channel:        channel,
		CurrentVersion: currentVersion,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("update check request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update check returned HTTP %d", resp.StatusCode)
	}

	var checkResult updateCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&checkResult); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if !checkResult.UpdateAvailable {
		fmt.Printf("[%s] Already up to date (version: %s)\n", component, currentVersion)
		return nil
	}

	release := checkResult.Release
	log.Printf("[update] Update available: %s → %s", currentVersion, release.Version)

	// Step 2: Download artifact.
	log.Printf("[update] Downloading artifact from %s", release.ArtifactURL)

	downloadPath := binaryPath + ".download"
	if err := downloadArtifact(release.ArtifactURL, downloadPath); err != nil {
		_ = os.Remove(downloadPath)
		return fmt.Errorf("download artifact: %w", err)
	}

	// Step 3: Verify SHA-256.
	log.Printf("[update] Verifying SHA-256 checksum...")

	computedHash, err := computeSHA256(downloadPath)
	if err != nil {
		_ = os.Remove(downloadPath)
		return fmt.Errorf("compute SHA-256: %w", err)
	}

	expectedHash := normalizeSHA256(release.SHA256)
	if computedHash != expectedHash {
		_ = os.Remove(downloadPath)
		log.Printf("[update] ERROR: SHA-256 mismatch! expected=%s got=%s", expectedHash, computedHash)
		return fmt.Errorf("SHA-256 mismatch: expected %s, got %s", expectedHash, computedHash)
	}

	log.Printf("[update] SHA-256 verified: %s", computedHash)

	// Step 4: Atomic binary replace — rename current → .bak, move new → current.
	backupPath := binaryPath + ".bak"
	log.Printf("[update] Creating backup: %s → %s", binaryPath, backupPath)

	if err := os.Rename(binaryPath, backupPath); err != nil {
		_ = os.Remove(downloadPath)
		return fmt.Errorf("backup current binary: %w", err)
	}

	if err := os.Rename(downloadPath, binaryPath); err != nil {
		// Restore backup if rename fails.
		_ = os.Rename(backupPath, binaryPath)
		return fmt.Errorf("replace binary: %w", err)
	}

	// Ensure the new binary is executable.
	if err := os.Chmod(binaryPath, 0o755); err != nil {
		log.Printf("[update] WARNING: chmod failed: %v", err)
	}

	// Step 5: Restart service.
	log.Printf("[update] Restarting service: %s", serviceName)

	if err := restartService(serviceName); err != nil {
		log.Printf("[update] ERROR: restart failed, rolling back: %v", err)
		_ = os.Rename(binaryPath, downloadPath)
		_ = os.Rename(backupPath, binaryPath)
		_ = restartService(serviceName)
		_ = os.Remove(downloadPath)
		return fmt.Errorf("restart service after update: %w", err)
	}

	// Step 6: Health check within 30 seconds.
	log.Printf("[update] Running health check (30s timeout)...")

	if err := healthCheck(serviceName, 30*time.Second); err != nil {
		log.Printf("[update] ERROR: health check failed, auto-rolling back: %v", err)
		// Auto rollback: restore .bak → current, restart service.
		_ = os.Remove(binaryPath)
		if renameErr := os.Rename(backupPath, binaryPath); renameErr != nil {
			log.Printf("[update] CRITICAL: rollback rename failed: %v", renameErr)
			return fmt.Errorf("health check failed and rollback failed: health=%w, rollback=%v", err, renameErr)
		}
		if restartErr := restartService(serviceName); restartErr != nil {
			log.Printf("[update] CRITICAL: rollback restart failed: %v", restartErr)
			return fmt.Errorf("health check failed and rollback restart failed: health=%w, restart=%v", err, restartErr)
		}
		log.Printf("[update] Rolled back to previous version successfully")
		return fmt.Errorf("health check failed after update, rolled back: %w", err)
	}

	// Step 7: Success — remove backup.
	log.Printf("[update] Update successful! %s → %s", currentVersion, release.Version)
	_ = os.Remove(backupPath)

	fmt.Printf("[%s] Successfully updated to version %s\n", component, release.Version)
	return nil
}

// Rollback restores the .bak binary and restarts the service.
func Rollback(binaryPath, serviceName string) error {
	if binaryPath == "" {
		return errors.New("binary path is required")
	}
	if serviceName == "" {
		return errors.New("service name is required")
	}

	backupPath := binaryPath + ".bak"

	// Check if backup exists.
	if _, err := os.Stat(backupPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no backup found at %s: nothing to rollback", backupPath)
		}
		return fmt.Errorf("check backup: %w", err)
	}

	log.Printf("[rollback] Restoring backup: %s → %s", backupPath, binaryPath)

	// Restore: rename .bak → current binary.
	if err := os.Rename(backupPath, binaryPath); err != nil {
		return fmt.Errorf("restore backup: %w", err)
	}

	// Restart service.
	log.Printf("[rollback] Restarting service: %s", serviceName)

	if err := restartService(serviceName); err != nil {
		return fmt.Errorf("restart service after rollback: %w", err)
	}

	log.Printf("[rollback] Rollback completed successfully")
	fmt.Printf("Rollback completed: restored %s and restarted %s\n", binaryPath, serviceName)
	return nil
}

// downloadArtifact downloads a file from the given URL to the output path.
func downloadArtifact(artifactURL, outputPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, artifactURL, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("write artifact: %w", err)
	}

	return nil
}

// computeSHA256 computes the SHA-256 hash of a file and returns it as a lowercase hex string.
func computeSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// normalizeSHA256 strips the "sha256:" prefix (case-insensitive) and lowercases the hash.
func normalizeSHA256(value string) string {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "sha256:") {
		value = value[7:]
	}
	return strings.ToLower(value)
}

// restartService restarts a systemd service using systemctl.
func restartService(serviceName string) error {
	cmd := exec.Command("systemctl", "restart", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl restart %s: %s: %w", serviceName, strings.TrimSpace(string(output)), err)
	}
	return nil
}

// healthCheck waits for the service to become healthy within the given timeout.
// It checks the service status via systemctl is-active every 2 seconds.
func healthCheck(serviceName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	interval := 2 * time.Second

	// Wait a moment for the service to start.
	time.Sleep(interval)

	for time.Now().Before(deadline) {
		cmd := exec.Command("systemctl", "is-active", serviceName)
		output, err := cmd.Output()
		status := strings.TrimSpace(string(output))

		if err == nil && status == "active" {
			return nil
		}

		time.Sleep(interval)
	}

	return fmt.Errorf("service %s did not become healthy within %s", serviceName, timeout)
}
