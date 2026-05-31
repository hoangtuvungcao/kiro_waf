package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	client "kiro_waf/internal/client"
	"kiro_waf/internal/client/xdp"
)

var configPath = flag.String("config", "/etc/kiro/kiro.yaml", "path to YAML config file")

func init() {
	// Register XDP startup hook to break import cycle between client and xdp packages.
	client.XDPStartupFunc = startXDPComponents
}

func main() {
	flag.Parse()
	os.Exit(client.RunWithConfig(*configPath))
}

// startXDPComponents initializes and starts XDP-related components:
// GeoIP loader with periodic refresh, botnet controller monitoring,
// and initial config sync to BPF maps.
func startXDPComponents(ctx context.Context, geoipCSVPath string) {
	// Start GeoIP loader
	geoipLoader := xdp.NewGeoIPLoader()
	if geoipCSVPath != "" {
		if err := geoipLoader.LoadFromCSV(geoipCSVPath); err != nil {
			log.Printf("WARNING: GeoIP CSV load failed: %v", err)
		}
	}
	if err := geoipLoader.LoadBlockedCountriesFromEnv(); err != nil {
		log.Printf("WARNING: GeoIP blocked countries load failed: %v", err)
	}
	geoipLoader.StartPeriodicRefresh(ctx, 24*time.Hour)

	// Start botnet controller
	botnetCtrl := xdp.NewBotnetController(xdp.BotnetControllerConfig{
		Threshold:       5000,
		CooldownSeconds: 30,
		PollInterval:    1 * time.Second,
	})
	botnetCtrl.Start(ctx)

	// Initialize ConfigSync and write initial XDP configuration to BPF map
	configSync := xdp.NewConfigSync(xdp.ConfigSyncOptions{})

	cfg := buildXDPConfigFromEnv()
	if err := configSync.SyncConfig(cfg); err != nil {
		log.Printf("ERROR: initial XDP config sync failed: %v", err)
	} else {
		log.Printf("xdp: initial config synced to BPF map successfully")
	}

	// Start SYN cookie key rotation
	configSync.StartKeyRotation(ctx)
}

// buildXDPConfigFromEnv constructs an XDPConfig from KIRO_XDP_* environment
// variables, falling back to sensible defaults matching the existing config pattern.
func buildXDPConfigFromEnv() xdp.XDPConfig {
	return xdp.XDPConfig{
		WindowNS:              envUint64("KIRO_XDP_WINDOW_NS", 1_000_000_000), // 1 second
		PerIPPPS:              envUint32("KIRO_XDP_PER_IP_PPS", 3000),
		SynPerIPPerSecond:     envUint32("KIRO_XDP_SYN_PER_IP_PER_SECOND", 200),
		UDPPerIPPerSecond:     envUint32("KIRO_XDP_UDP_PER_IP_PER_SECOND", 500),
		ICMPPerIPPerSecond:    envUint32("KIRO_XDP_ICMP_PER_IP_PER_SECOND", 30),
		DropPrivateSourceIP:   envUint8Bool("KIRO_XDP_DROP_PRIVATE_SOURCE_IP", 1),
		DropMalformed:         envUint8Bool("KIRO_XDP_DROP_MALFORMED", 1),
		RateLimitEnabled:      envUint8Bool("KIRO_XDP_RATE_LIMIT_ENABLED", 1),
		DropFragments:         envUint8Bool("KIRO_XDP_DROP_FRAGMENTS", 1),
		PerSubnet24PPS:        envUint32("KIRO_XDP_PER_SUBNET24_PPS", 30000),
		SynCookieThreshold:    envUint32("KIRO_XDP_SYN_COOKIE_THRESHOLD", 10000),
		SynCookieActive:       envUint8Bool("KIRO_XDP_SYN_COOKIE_ACTIVE", 0),
		ConnTrackerEnabled:    envUint8Bool("KIRO_XDP_CONN_TRACKER_ENABLED", 1),
		GeoIPEnabled:          envUint8Bool("KIRO_XDP_GEOIP_ENABLED", 1),
		BotnetNewIPThreshold:  envUint32("KIRO_XDP_BOTNET_NEW_IP_THRESHOLD", 5000),
		BotnetCooldownSeconds: envUint32("KIRO_XDP_BOTNET_COOLDOWN_SECONDS", 30),
		BotnetModeActive:      envUint8Bool("KIRO_XDP_BOTNET_MODE_ACTIVE", 0),
	}
}

// envUint64 reads an environment variable as uint64, returning fallback if unset or invalid.
func envUint64(key string, fallback uint64) uint64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

// envUint32 reads an environment variable as uint32, returning fallback if unset or invalid.
func envUint32(key string, fallback uint32) uint32 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseUint(v, 10, 32)
	if err != nil {
		return fallback
	}
	return uint32(n)
}

// envUint8Bool reads an environment variable as a boolean-like uint8 (0 or 1).
// Accepts "1", "true", "yes" as 1; "0", "false", "no" as 0.
func envUint8Bool(key string, fallback uint8) uint8 {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "yes":
		return 1
	case "0", "false", "no":
		return 0
	default:
		return fallback
	}
}
