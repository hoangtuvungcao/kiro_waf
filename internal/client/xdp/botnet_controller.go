// Package xdp provides userspace utilities for managing XDP/eBPF maps.
package xdp

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// BotnetController monitors the new_ip_rate per-CPU array map from userspace
// and manages botnet mode deactivation. It periodically reads per-CPU counters,
// sums them to get the true global new-IP rate, and when the rate stays below
// 50% of the configured threshold for the cooldown period, it writes
// botnet_mode_active = 0 to the kiro_config map.
//
// Requirements: 15.4, 15.6
type BotnetController struct {
	// configMapName is the BPF map name for the kiro_config array.
	configMapName string

	// rateMapName is the BPF map name for the new_ip_rate per-CPU array.
	rateMapName string

	// threshold is the botnet_new_ip_threshold (new IPs/s to activate botnet mode).
	threshold uint32

	// cooldownDuration is how long the rate must stay below 50% threshold
	// before botnet mode is deactivated.
	cooldownDuration time.Duration

	// pollInterval is how often the controller reads the per-CPU counters.
	pollInterval time.Duration

	mu              sync.Mutex
	belowSince      time.Time // when rate first dropped below 50% threshold
	belowThreshold  bool      // whether rate is currently below 50%
	lastRate        uint32    // last observed summed rate
	botnetActive    bool      // cached botnet mode state
}

// BotnetControllerConfig holds configuration for the BotnetController.
type BotnetControllerConfig struct {
	// Threshold is the botnet_new_ip_threshold (new IPs/s). Default: 5000.
	Threshold uint32

	// CooldownSeconds is how long (in seconds) the rate must stay below
	// 50% of threshold before deactivating botnet mode. Default: 30.
	CooldownSeconds uint32

	// PollInterval is how often to read per-CPU counters. Default: 1s.
	PollInterval time.Duration
}

// NewBotnetController creates a new BotnetController with the given configuration.
func NewBotnetController(cfg BotnetControllerConfig) *BotnetController {
	if cfg.Threshold == 0 {
		cfg.Threshold = 5000
	}
	if cfg.CooldownSeconds == 0 {
		cfg.CooldownSeconds = 30
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 1 * time.Second
	}

	return &BotnetController{
		configMapName:    "kiro_config",
		rateMapName:      "new_ip_rate",
		threshold:        cfg.Threshold,
		cooldownDuration: time.Duration(cfg.CooldownSeconds) * time.Second,
		pollInterval:     cfg.PollInterval,
	}
}

// Start begins the background monitoring loop. It periodically reads the
// new_ip_rate per-CPU array map, sums all CPU counters, and when the rate
// stays below 50% of the threshold for the configured cooldown period,
// it deactivates botnet mode by writing botnet_mode_active = 0 to kiro_config.
//
// The goroutine exits when the context is cancelled.
func (bc *BotnetController) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(bc.pollInterval)
		defer ticker.Stop()

		log.Printf("botnet_controller: started monitoring (threshold=%d, cooldown=%s, poll=%s)",
			bc.threshold, bc.cooldownDuration, bc.pollInterval)

		for {
			select {
			case <-ctx.Done():
				log.Printf("botnet_controller: stopped")
				return
			case <-ticker.C:
				bc.poll()
			}
		}
	}()
}

// ForceDisable manually overrides botnet mode by writing botnet_mode_active = 0
// to the kiro_config BPF map. This provides an escape hatch for false positives.
func (bc *BotnetController) ForceDisable() error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if err := bc.writeBotnetModeActive(0); err != nil {
		return fmt.Errorf("botnet_controller: force disable failed: %w", err)
	}

	bc.botnetActive = false
	bc.belowThreshold = false
	log.Printf("botnet_controller: botnet mode force-disabled")
	return nil
}

// LastRate returns the last observed summed new-IP rate across all CPUs.
func (bc *BotnetController) LastRate() uint32 {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return bc.lastRate
}

// IsBotnetActive returns the cached botnet mode state.
func (bc *BotnetController) IsBotnetActive() bool {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return bc.botnetActive
}

// poll reads the per-CPU counters, sums them, and checks cooldown logic.
func (bc *BotnetController) poll() {
	rate, err := bc.readNewIPRate()
	if err != nil {
		log.Printf("botnet_controller: failed to read new_ip_rate: %v", err)
		return
	}

	bc.mu.Lock()
	defer bc.mu.Unlock()

	bc.lastRate = rate
	halfThreshold := bc.threshold / 2

	if rate < halfThreshold {
		// Rate is below 50% threshold
		if !bc.belowThreshold {
			// Just dropped below — start tracking cooldown
			bc.belowThreshold = true
			bc.belowSince = time.Now()
		} else {
			// Already below — check if cooldown period has elapsed
			if time.Since(bc.belowSince) >= bc.cooldownDuration {
				// Cooldown elapsed — deactivate botnet mode
				if bc.botnetActive {
					if err := bc.writeBotnetModeActive(0); err != nil {
						log.Printf("botnet_controller: failed to deactivate: %v", err)
					} else {
						bc.botnetActive = false
						log.Printf("botnet_controller: botnet mode deactivated (rate=%d, threshold_50%%=%d, cooldown=%s)",
							rate, halfThreshold, bc.cooldownDuration)
					}
				}
			}
		}
	} else {
		// Rate is at or above 50% threshold — reset cooldown tracking
		bc.belowThreshold = false
		bc.botnetActive = true
	}
}

// readNewIPRate reads the new_ip_rate per-CPU array map and sums all CPU
// counters to get the true global new-IP rate.
//
// Uses bpftool to read per-CPU values:
//   bpftool map lookup percpu name new_ip_rate key 0x00 0x00 0x00 0x00
//
// The output contains one value per CPU with the new_ip_counter struct:
//   { window_start_ns (8 bytes), count (4 bytes), _pad (4 bytes) }
func (bc *BotnetController) readNewIPRate() (uint32, error) {
	keyHex := "0x00 0x00 0x00 0x00"

	args := []string{"map", "lookup", "percpu", "name", bc.rateMapName, "key"}
	args = append(args, strings.Fields(keyHex)...)

	cmd := exec.Command("bpftool", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("bpftool map lookup percpu %s: %s: %w",
			bc.rateMapName, strings.TrimSpace(string(output)), err)
	}

	return parsePerCPUCounters(string(output))
}

// parsePerCPUCounters parses bpftool percpu output and sums the count fields.
//
// Expected bpftool output format (one entry per CPU):
//
//	{
//	    "key": 0,
//	    "values": [{
//	        "cpu": 0,
//	        "value": {
//	            "": "0x00 0x00 ... (16 bytes hex)"
//	        }
//	    }, ...]
//	}
//
// Or the simpler raw hex format per CPU line:
//
//	cpu0: 00 00 00 00 00 00 00 00 e8 03 00 00 00 00 00 00
//	cpu1: 00 00 00 00 00 00 00 00 f4 01 00 00 00 00 00 00
//
// We parse both formats by looking for hex byte sequences representing
// the new_ip_counter struct: { window_start_ns(8B), count(4B), _pad(4B) }
func parsePerCPUCounters(output string) (uint32, error) {
	var totalCount uint32

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Try to parse "cpuN:" format (raw hex dump from bpftool)
		if idx := strings.Index(line, ":"); idx > 0 {
			prefix := line[:idx]
			if strings.HasPrefix(strings.TrimSpace(prefix), "cpu") {
				hexPart := strings.TrimSpace(line[idx+1:])
				count, err := extractCountFromHex(hexPart)
				if err == nil {
					totalCount += count
				}
				continue
			}
		}

		// Try to parse JSON-style "value" lines with hex bytes
		// Look for lines containing hex byte sequences (0x prefixed)
		if strings.Contains(line, "0x") {
			hexPart := extractHexValue(line)
			if hexPart != "" {
				count, err := extractCountFromHex(hexPart)
				if err == nil {
					totalCount += count
				}
			}
		}
	}

	return totalCount, nil
}

// extractCountFromHex parses a hex byte string representing the new_ip_counter
// struct and extracts the count field (bytes 8-11, little-endian uint32).
//
// Struct layout: { window_start_ns(8B LE), count(4B LE), _pad(4B) }
// Total: 16 bytes
func extractCountFromHex(hexStr string) (uint32, error) {
	// Parse hex bytes (space-separated, with or without 0x prefix)
	hexStr = strings.TrimSpace(hexStr)
	parts := strings.Fields(hexStr)

	// Clean up hex parts (remove 0x prefix if present)
	var bytes []byte
	for _, p := range parts {
		p = strings.TrimPrefix(p, "0x")
		if len(p) == 2 {
			b, err := strconv.ParseUint(p, 16, 8)
			if err != nil {
				continue
			}
			bytes = append(bytes, byte(b))
		}
	}

	// Need at least 12 bytes to read the count field (offset 8, size 4)
	if len(bytes) < 12 {
		return 0, fmt.Errorf("insufficient bytes: got %d, need 12", len(bytes))
	}

	// Extract count field at offset 8 (little-endian uint32)
	count := binary.LittleEndian.Uint32(bytes[8:12])
	return count, nil
}

// extractHexValue extracts a hex byte sequence from a JSON-style value line.
// Looks for content between quotes that contains hex bytes.
func extractHexValue(line string) string {
	// Look for quoted hex values like "0x00 0x00 ..."
	start := strings.Index(line, "\"")
	if start == -1 {
		return ""
	}
	end := strings.LastIndex(line, "\"")
	if end <= start {
		return ""
	}
	return line[start+1 : end]
}

// writeBotnetModeActive writes the botnet_mode_active field in the kiro_config
// BPF array map. The field is at a specific byte offset within the
// kiro_xdp_config struct.
//
// Struct layout (matching C struct kiro_xdp_config):
//   offset 0:  window_ns (8 bytes)
//   offset 8:  per_ip_pps (4 bytes)
//   offset 12: syn_per_ip_per_second (4 bytes)
//   offset 16: udp_per_ip_per_second (4 bytes)
//   offset 20: icmp_per_ip_per_second (4 bytes)
//   offset 24: drop_private_source_ip (1 byte)
//   offset 25: drop_malformed (1 byte)
//   offset 26: rate_limit_enabled (1 byte)
//   offset 27: drop_fragments (1 byte)
//   offset 28: per_subnet24_pps (4 bytes)
//   offset 32: syn_cookie_threshold (4 bytes)
//   offset 36: syn_cookie_active (1 byte)
//   offset 37: conn_tracker_enabled (1 byte)
//   offset 38: geoip_enabled (1 byte)
//   offset 39: _pad_sc (1 byte)
//   offset 40: botnet_new_ip_threshold (4 bytes)
//   offset 44: botnet_cooldown_seconds (4 bytes)
//   offset 48: botnet_mode_active (1 byte)
//   offset 49: _pad_bn[3] (3 bytes)
//
// To update a single field, we must read the full config, modify the field,
// and write it back. We use bpftool for this.
func (bc *BotnetController) writeBotnetModeActive(value uint8) error {
	// First, read the current config
	configBytes, err := bc.readConfigMap()
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	// Modify the botnet_mode_active field at offset 48
	const botnetModeActiveOffset = 48
	if len(configBytes) <= botnetModeActiveOffset {
		return fmt.Errorf("config too short: %d bytes, need at least %d",
			len(configBytes), botnetModeActiveOffset+1)
	}
	configBytes[botnetModeActiveOffset] = value

	// Write back the full config
	return bc.writeConfigMap(configBytes)
}

// readConfigMap reads the full kiro_config map value using bpftool.
func (bc *BotnetController) readConfigMap() ([]byte, error) {
	keyHex := "0x00 0x00 0x00 0x00"

	args := []string{"map", "lookup", "name", bc.configMapName, "key"}
	args = append(args, strings.Fields(keyHex)...)

	cmd := exec.Command("bpftool", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("bpftool map lookup %s: %s: %w",
			bc.configMapName, strings.TrimSpace(string(output)), err)
	}

	return parseHexBytes(string(output))
}

// writeConfigMap writes the full kiro_config map value using bpftool.
func (bc *BotnetController) writeConfigMap(data []byte) error {
	keyHex := "0x00 0x00 0x00 0x00"
	valueHex := bytesToHex(data)

	args := []string{"map", "update", "name", bc.configMapName}
	args = append(args, "key")
	args = append(args, strings.Fields(keyHex)...)
	args = append(args, "value")
	args = append(args, strings.Fields(valueHex)...)

	cmd := exec.Command("bpftool", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bpftool map update %s: %s: %w",
			bc.configMapName, strings.TrimSpace(string(output)), err)
	}
	return nil
}

// parseHexBytes parses bpftool output containing hex byte values.
// Handles both JSON format and raw hex dump format.
func parseHexBytes(output string) ([]byte, error) {
	var result []byte

	// Try to find hex bytes in the output
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for "value:" prefix (bpftool raw format)
		if idx := strings.Index(line, "value:"); idx >= 0 {
			hexPart := strings.TrimSpace(line[idx+6:])
			bytes, err := parseHexString(hexPart)
			if err == nil && len(bytes) > 0 {
				result = append(result, bytes...)
			}
			continue
		}

		// Look for quoted hex values in JSON format
		if strings.Contains(line, "0x") {
			hexVal := extractHexValue(line)
			if hexVal != "" {
				bytes, err := parseHexString(hexVal)
				if err == nil && len(bytes) > 0 {
					result = append(result, bytes...)
				}
				continue
			}

			// Try parsing the line directly as hex bytes
			bytes, err := parseHexString(line)
			if err == nil && len(bytes) > 0 {
				result = append(result, bytes...)
			}
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no hex bytes found in output: %s", strings.TrimSpace(output))
	}

	return result, nil
}

// parseHexString parses a space-separated hex byte string into a byte slice.
func parseHexString(hexStr string) ([]byte, error) {
	parts := strings.Fields(hexStr)
	var result []byte

	for _, p := range parts {
		p = strings.TrimPrefix(p, "0x")
		if len(p) == 2 {
			b, err := strconv.ParseUint(p, 16, 8)
			if err != nil {
				continue
			}
			result = append(result, byte(b))
		}
	}

	return result, nil
}

// bytesToHex converts a byte slice to space-separated hex string (0x prefixed).
func bytesToHex(data []byte) string {
	parts := make([]string, len(data))
	for i, b := range data {
		parts[i] = fmt.Sprintf("0x%02x", b)
	}
	return strings.Join(parts, " ")
}
