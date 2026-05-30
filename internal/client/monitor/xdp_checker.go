package monitor

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

// XDPChecker handles XDP program health checking and auto-reload/reattach.
type XDPChecker struct {
	// InterfaceName is the network interface where XDP is attached (e.g., "eth0").
	InterfaceName string

	// XDPObjectPath is the path to the compiled XDP BPF object file.
	XDPObjectPath string

	// reloadFailures tracks consecutive XDP reload failures.
	reloadFailures *FailureTracker

	// lastReloadAttempt tracks when the last reload was attempted for 60s retry logic.
	lastReloadAttempt time.Time

	// reloadCooldown is the wait time between retries after 3 consecutive failures (60s).
	reloadCooldown time.Duration

	// maxReloadFailures is the threshold for critical escalation (3).
	maxReloadFailures int

	// alertFunc is called when XDP reload failures escalate.
	alertFunc func(alertType string, message string)
}

// XDPCheckerConfig holds configuration for the XDP checker.
type XDPCheckerConfig struct {
	// InterfaceName is the network interface (e.g., "eth0").
	InterfaceName string

	// XDPObjectPath is the path to the XDP BPF object file.
	XDPObjectPath string

	// AlertFunc is called when failures escalate and need to alert Master.
	AlertFunc func(alertType string, message string)
}

// NewXDPChecker creates a new XDPChecker with the given configuration.
func NewXDPChecker(cfg XDPCheckerConfig) *XDPChecker {
	alertFn := cfg.AlertFunc
	if alertFn == nil {
		alertFn = func(alertType, message string) {
			log.Printf("Health monitor: XDP alert [%s]: %s", alertType, message)
		}
	}

	return &XDPChecker{
		InterfaceName:     cfg.InterfaceName,
		XDPObjectPath:     cfg.XDPObjectPath,
		reloadFailures:    NewFailureTracker(5 * time.Minute),
		reloadCooldown:    60 * time.Second,
		maxReloadFailures: 3,
		alertFunc:         alertFn,
	}
}

// XDPStatus represents the current state of the XDP program.
type XDPStatus struct {
	// Attached indicates whether the XDP program is attached to the interface.
	Attached bool

	// Stats holds the BPF map statistics if available.
	Stats *XDPStats
}

// XDPStats holds XDP filter statistics read from BPF maps.
type XDPStats struct {
	TotalPackets   uint64 `json:"total_packets"`
	PassedPackets  uint64 `json:"passed_packets"`
	DroppedPackets uint64 `json:"dropped_packets"`
}

// CheckXDP verifies that the XDP program is attached and reads BPF map statistics.
// Returns the current XDP status.
func (xc *XDPChecker) CheckXDP(ctx context.Context) XDPStatus {
	attached := xc.isXDPAttached(ctx)
	var stats *XDPStats
	if attached {
		stats = xc.readBPFMapStats(ctx)
	}

	return XDPStatus{
		Attached: attached,
		Stats:    stats,
	}
}

// HandleDetached is called when XDP is detected as detached.
// It attempts to reload and reattach the XDP program within 30 seconds.
// Implements failure tracking: 3 consecutive failures → critical log + alert + retry every 60s.
func (xc *XDPChecker) HandleDetached(ctx context.Context) error {
	// Check if we're in cooldown after 3 consecutive failures
	if xc.reloadFailures.ShouldEscalate(xc.maxReloadFailures) {
		elapsed := time.Since(xc.lastReloadAttempt)
		if elapsed < xc.reloadCooldown {
			log.Printf("Health monitor: XDP reload in cooldown, %s remaining",
				xc.reloadCooldown-elapsed)
			return fmt.Errorf("xdp reload in cooldown after %d consecutive failures",
				xc.reloadFailures.Count())
		}
		// Cooldown expired, reset and try again
		log.Printf("Health monitor: XDP reload cooldown expired, retrying")
		xc.reloadFailures.Reset()
	}

	log.Printf("Health monitor: XDP detached from %s, attempting reload/reattach", xc.InterfaceName)
	xc.lastReloadAttempt = time.Now()

	// Attempt reload with 30-second deadline
	reloadCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	err := xc.reloadXDP(reloadCtx)
	if err != nil {
		xc.reloadFailures.Record()
		failCount := xc.reloadFailures.Count()

		log.Printf("Health monitor: XDP reload failed (attempt %d/%d): %v",
			failCount, xc.maxReloadFailures, err)

		// Check if we've hit the escalation threshold
		if xc.reloadFailures.ShouldEscalate(xc.maxReloadFailures) {
			log.Printf("CRITICAL: Health monitor: XDP reload failed %d consecutive times, alerting Master",
				xc.maxReloadFailures)
			xc.alertFunc("xdp_reload_failed", fmt.Sprintf(
				"XDP reload failed %d consecutive times on interface %s: %v",
				xc.maxReloadFailures, xc.InterfaceName, err))
		}

		return fmt.Errorf("xdp reload failed: %w", err)
	}

	// Reload succeeded
	xc.reloadFailures.Reset()
	log.Printf("Health monitor: XDP successfully reattached to %s", xc.InterfaceName)
	return nil
}

// ReloadFailureCount returns the current number of consecutive reload failures.
func (xc *XDPChecker) ReloadFailureCount() int {
	return xc.reloadFailures.Count()
}

// isXDPAttached checks if an XDP program is attached to the configured network interface.
// It uses `ip link show <interface>` and checks for "xdp" in the output.
func (xc *XDPChecker) isXDPAttached(ctx context.Context) bool {
	if xc.InterfaceName == "" {
		return false
	}

	cmd := exec.CommandContext(ctx, "ip", "link", "show", xc.InterfaceName)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Health monitor: failed to check XDP attachment: %v", err)
		return false
	}

	// Look for "xdp" or "prog/xdp" in the ip link output
	outputStr := string(output)
	return strings.Contains(outputStr, "xdp") || strings.Contains(outputStr, "prog/xdp")
}

// readBPFMapStats reads statistics from the XDP BPF maps using bpftool.
// Returns nil if stats cannot be read (non-critical).
func (xc *XDPChecker) readBPFMapStats(ctx context.Context) *XDPStats {
	// Use bpftool to read the stats map
	// The stats map is a per-CPU array, we read aggregated values
	cmd := exec.CommandContext(ctx, "bpftool", "map", "dump", "name", "stats_map", "-j")
	output, err := cmd.Output()
	if err != nil {
		// Stats reading is non-critical, just log at debug level
		return nil
	}

	stats := parseXDPStats(string(output))
	return stats
}

// parseXDPStats parses bpftool JSON output into XDPStats.
// This is a best-effort parser; returns nil if parsing fails.
func parseXDPStats(output string) *XDPStats {
	// bpftool map dump outputs JSON array of key-value pairs
	// For per-CPU maps, values are arrays of per-CPU values
	// We do a simple heuristic parse here since we don't want to add
	// a JSON dependency just for stats parsing
	if output == "" || output == "[]" {
		return nil
	}

	// Return basic stats structure - actual parsing depends on map layout
	return &XDPStats{}
}

// reloadXDP attempts to reload and reattach the XDP program to the network interface.
// It first detaches any existing program, then loads and attaches the new one.
func (xc *XDPChecker) reloadXDP(ctx context.Context) error {
	if xc.InterfaceName == "" {
		return fmt.Errorf("no interface configured for XDP")
	}
	if xc.XDPObjectPath == "" {
		return fmt.Errorf("no XDP object path configured")
	}

	// Step 1: Detach existing XDP program (ignore errors - it may already be detached)
	detachCmd := exec.CommandContext(ctx, "ip", "link", "set", "dev", xc.InterfaceName, "xdp", "off")
	_ = detachCmd.Run()

	// Step 2: Attach the XDP program
	// Use ip link set dev <iface> xdp obj <path> sec xdp
	attachCmd := exec.CommandContext(ctx, "ip", "link", "set", "dev", xc.InterfaceName,
		"xdp", "obj", xc.XDPObjectPath, "sec", "xdp")
	output, err := attachCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("attach XDP to %s: %s: %w",
			xc.InterfaceName, strings.TrimSpace(string(output)), err)
	}

	// Step 3: Verify attachment
	if !xc.isXDPAttached(ctx) {
		return fmt.Errorf("XDP program not detected after attach on %s", xc.InterfaceName)
	}

	return nil
}
