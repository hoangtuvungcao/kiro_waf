package proxy

import (
	"log"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
)

const (
	// DefaultGOGC is the default garbage collection target percentage.
	// A higher value reduces GC frequency at the cost of more memory usage.
	// 200 means GC triggers when heap grows to 2x the live heap size.
	DefaultGOGC = 200

	// DefaultGOMEMLIMIT is the default soft memory limit (512MB).
	// This helps the GC make better decisions about when to collect.
	DefaultGOMEMLIMIT = 512 * 1024 * 1024 // 512MB
)

// GCConfig holds garbage collection tuning parameters.
type GCConfig struct {
	// GOGC is the garbage collection target percentage.
	// Higher values reduce GC frequency. -1 disables GC.
	// Read from GOGC environment variable.
	GOGC int

	// GOMEMLIMIT is the soft memory limit in bytes.
	// The runtime will try to keep memory usage below this limit.
	// Read from GOMEMLIMIT environment variable.
	// Supports suffixes: B, KB, KiB, MB, MiB, GB, GiB.
	GOMEMLIMIT int64
}

// ConfigureGC reads GOGC and GOMEMLIMIT from environment variables and applies them.
// This should be called early in application startup to ensure GC is tuned
// before significant allocation occurs.
//
// Environment variables:
//   - GOGC: integer percentage (e.g., "200" for 2x live heap trigger)
//   - GOMEMLIMIT: memory limit with optional suffix (e.g., "512MiB", "1GiB", "536870912")
//
// If environment variables are not set, defaults are applied:
//   - GOGC=200 (less frequent GC, suitable for high-throughput proxy)
//   - GOMEMLIMIT=512MiB (matches requirement 7.2: <512MB RSS under load)
func ConfigureGC() GCConfig {
	cfg := GCConfig{
		GOGC:       DefaultGOGC,
		GOMEMLIMIT: DefaultGOMEMLIMIT,
	}

	// Read GOGC from environment
	if gogcStr := strings.TrimSpace(os.Getenv("GOGC")); gogcStr != "" {
		if v, err := strconv.Atoi(gogcStr); err == nil {
			cfg.GOGC = v
		} else {
			log.Printf("proxy/gc: invalid GOGC value %q, using default %d", gogcStr, DefaultGOGC)
		}
	}

	// Read GOMEMLIMIT from environment
	if memlimitStr := strings.TrimSpace(os.Getenv("GOMEMLIMIT")); memlimitStr != "" {
		if v, err := parseMemLimit(memlimitStr); err == nil {
			cfg.GOMEMLIMIT = v
		} else {
			log.Printf("proxy/gc: invalid GOMEMLIMIT value %q, using default %d", memlimitStr, DefaultGOMEMLIMIT)
		}
	}

	// Apply settings
	debug.SetGCPercent(cfg.GOGC)
	debug.SetMemoryLimit(cfg.GOMEMLIMIT)

	log.Printf("proxy/gc: configured GOGC=%d GOMEMLIMIT=%d bytes", cfg.GOGC, cfg.GOMEMLIMIT)

	return cfg
}

// parseMemLimit parses a memory limit string with optional suffix.
// Supported formats: "512", "512B", "512KB", "512KiB", "512MB", "512MiB", "512GB", "512GiB"
func parseMemLimit(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, strconv.ErrSyntax
	}

	// Find where the numeric part ends
	numEnd := 0
	for numEnd < len(s) && (s[numEnd] >= '0' && s[numEnd] <= '9' || s[numEnd] == '.') {
		numEnd++
	}

	if numEnd == 0 {
		return 0, strconv.ErrSyntax
	}

	numStr := s[:numEnd]
	suffix := strings.TrimSpace(s[numEnd:])

	val, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		// Try parsing as float for cases like "1.5GiB"
		fval, ferr := strconv.ParseFloat(numStr, 64)
		if ferr != nil {
			return 0, err
		}
		multiplier := suffixMultiplier(suffix)
		return int64(fval * float64(multiplier)), nil
	}

	multiplier := suffixMultiplier(suffix)
	return val * multiplier, nil
}

// suffixMultiplier returns the byte multiplier for a memory size suffix.
func suffixMultiplier(suffix string) int64 {
	switch strings.ToUpper(suffix) {
	case "", "B":
		return 1
	case "KB":
		return 1000
	case "KIB":
		return 1024
	case "MB":
		return 1000 * 1000
	case "MIB":
		return 1024 * 1024
	case "GB":
		return 1000 * 1000 * 1000
	case "GIB":
		return 1024 * 1024 * 1024
	default:
		return 1
	}
}
