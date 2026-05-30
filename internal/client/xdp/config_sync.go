// Package xdp provides userspace utilities for managing XDP/eBPF maps.
package xdp

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log"
	"sync"
	"time"
)

// XDPConfig is the Go-side representation of the C struct kiro_xdp_config.
// It matches the BPF map value layout exactly for serialization to the
// kiro_config BPF array map.
//
// This struct mirrors the fields in internal/client.XDPConfig but is defined
// here to avoid import cycles (the parent client package imports xdp).
type XDPConfig struct {
	WindowNS              uint64
	PerIPPPS              uint32
	SynPerIPPerSecond     uint32
	UDPPerIPPerSecond     uint32
	ICMPPerIPPerSecond    uint32
	DropPrivateSourceIP   uint8
	DropMalformed         uint8
	RateLimitEnabled      uint8
	DropFragments         uint8
	PerSubnet24PPS        uint32
	SynCookieThreshold    uint32
	SynCookieActive       uint8
	ConnTrackerEnabled    uint8
	GeoIPEnabled          uint8
	PadSC                 uint8
	BotnetNewIPThreshold  uint32
	BotnetCooldownSeconds uint32
	BotnetModeActive      uint8
	PadBN                 [3]uint8
}

// ConfigSync manages synchronization of the Go-side XDPConfig struct
// to the BPF kiro_config array map. It also handles SYN cookie key
// rotation (every 24h) via the syn_cookie_key_map BPF array map.
//
// Requirements: 12.1, 14.6, 15.6
type ConfigSync struct {
	// configMapName is the BPF map name for the kiro_config array.
	configMapName string

	// synCookieKeyMapName is the BPF map name for the SYN cookie key.
	synCookieKeyMapName string

	// keyRotationInterval is how often the SYN cookie key is rotated.
	keyRotationInterval time.Duration

	mu         sync.Mutex
	lastSync   time.Time
	lastKey    synCookieKey
	keyRotated bool
}

// synCookieKey holds the SipHash-2-4 key material for SYN cookie computation.
// Matches the C struct syn_cookie_key: { __u64 k0; __u64 k1; }
type synCookieKey struct {
	K0 uint64
	K1 uint64
}

// ConfigSyncOptions holds configuration for the ConfigSync instance.
type ConfigSyncOptions struct {
	// KeyRotationInterval is how often the SYN cookie key is rotated.
	// Default: 24 hours.
	KeyRotationInterval time.Duration
}

// NewConfigSync creates a new ConfigSync instance.
func NewConfigSync(opts ConfigSyncOptions) *ConfigSync {
	if opts.KeyRotationInterval <= 0 {
		opts.KeyRotationInterval = 24 * time.Hour
	}

	return &ConfigSync{
		configMapName:       "kiro_config",
		synCookieKeyMapName: "syn_cookie_key_map",
		keyRotationInterval: opts.KeyRotationInterval,
	}
}

// SyncConfig writes the full XDPConfig struct to the kiro_config BPF array map
// at index 0. The struct is serialized to match the C struct layout exactly.
//
// C struct kiro_xdp_config layout (52 bytes total):
//
//	offset  0: window_ns              (uint64, 8 bytes)
//	offset  8: per_ip_pps             (uint32, 4 bytes)
//	offset 12: syn_per_ip_per_second  (uint32, 4 bytes)
//	offset 16: udp_per_ip_per_second  (uint32, 4 bytes)
//	offset 20: icmp_per_ip_per_second (uint32, 4 bytes)
//	offset 24: drop_private_source_ip (uint8, 1 byte)
//	offset 25: drop_malformed         (uint8, 1 byte)
//	offset 26: rate_limit_enabled     (uint8, 1 byte)
//	offset 27: drop_fragments         (uint8, 1 byte)
//	offset 28: per_subnet24_pps       (uint32, 4 bytes)
//	offset 32: syn_cookie_threshold   (uint32, 4 bytes)
//	offset 36: syn_cookie_active      (uint8, 1 byte)
//	offset 37: conn_tracker_enabled   (uint8, 1 byte)
//	offset 38: geoip_enabled          (uint8, 1 byte)
//	offset 39: _pad_sc                (uint8, 1 byte)
//	offset 40: botnet_new_ip_threshold  (uint32, 4 bytes)
//	offset 44: botnet_cooldown_seconds  (uint32, 4 bytes)
//	offset 48: botnet_mode_active       (uint8, 1 byte)
//	offset 49: _pad_bn[3]              (3 bytes)
//	Total: 52 bytes
func (cs *ConfigSync) SyncConfig(cfg XDPConfig) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	data := SerializeXDPConfig(cfg)

	if err := cs.writeConfigToMap(data); err != nil {
		return fmt.Errorf("config_sync: write config: %w", err)
	}

	cs.lastSync = time.Now()
	log.Printf("config_sync: synced XDP config to BPF map (%d bytes)", len(data))
	return nil
}

// SerializeXDPConfig converts the Go XDPConfig struct to a byte slice
// matching the C struct kiro_xdp_config layout (little-endian).
func SerializeXDPConfig(cfg XDPConfig) []byte {
	buf := make([]byte, 52)

	// offset 0: window_ns (uint64)
	binary.LittleEndian.PutUint64(buf[0:8], cfg.WindowNS)
	// offset 8: per_ip_pps (uint32)
	binary.LittleEndian.PutUint32(buf[8:12], cfg.PerIPPPS)
	// offset 12: syn_per_ip_per_second (uint32)
	binary.LittleEndian.PutUint32(buf[12:16], cfg.SynPerIPPerSecond)
	// offset 16: udp_per_ip_per_second (uint32)
	binary.LittleEndian.PutUint32(buf[16:20], cfg.UDPPerIPPerSecond)
	// offset 20: icmp_per_ip_per_second (uint32)
	binary.LittleEndian.PutUint32(buf[20:24], cfg.ICMPPerIPPerSecond)
	// offset 24: drop_private_source_ip (uint8)
	buf[24] = cfg.DropPrivateSourceIP
	// offset 25: drop_malformed (uint8)
	buf[25] = cfg.DropMalformed
	// offset 26: rate_limit_enabled (uint8)
	buf[26] = cfg.RateLimitEnabled
	// offset 27: drop_fragments (uint8)
	buf[27] = cfg.DropFragments
	// offset 28: per_subnet24_pps (uint32)
	binary.LittleEndian.PutUint32(buf[28:32], cfg.PerSubnet24PPS)
	// offset 32: syn_cookie_threshold (uint32)
	binary.LittleEndian.PutUint32(buf[32:36], cfg.SynCookieThreshold)
	// offset 36: syn_cookie_active (uint8)
	buf[36] = cfg.SynCookieActive
	// offset 37: conn_tracker_enabled (uint8)
	buf[37] = cfg.ConnTrackerEnabled
	// offset 38: geoip_enabled (uint8)
	buf[38] = cfg.GeoIPEnabled
	// offset 39: _pad_sc (uint8)
	buf[39] = cfg.PadSC
	// offset 40: botnet_new_ip_threshold (uint32)
	binary.LittleEndian.PutUint32(buf[40:44], cfg.BotnetNewIPThreshold)
	// offset 44: botnet_cooldown_seconds (uint32)
	binary.LittleEndian.PutUint32(buf[44:48], cfg.BotnetCooldownSeconds)
	// offset 48: botnet_mode_active (uint8)
	buf[48] = cfg.BotnetModeActive
	// offset 49-51: _pad_bn[3]
	buf[49] = cfg.PadBN[0]
	buf[50] = cfg.PadBN[1]
	buf[51] = cfg.PadBN[2]

	return buf
}

// DeserializeXDPConfig converts a byte slice from the BPF map back to
// the Go XDPConfig struct.
func DeserializeXDPConfig(data []byte) (XDPConfig, error) {
	if len(data) < 52 {
		return XDPConfig{}, fmt.Errorf("config data too short: %d bytes, need 52", len(data))
	}

	cfg := XDPConfig{
		WindowNS:              binary.LittleEndian.Uint64(data[0:8]),
		PerIPPPS:              binary.LittleEndian.Uint32(data[8:12]),
		SynPerIPPerSecond:     binary.LittleEndian.Uint32(data[12:16]),
		UDPPerIPPerSecond:     binary.LittleEndian.Uint32(data[16:20]),
		ICMPPerIPPerSecond:    binary.LittleEndian.Uint32(data[20:24]),
		DropPrivateSourceIP:   data[24],
		DropMalformed:         data[25],
		RateLimitEnabled:      data[26],
		DropFragments:         data[27],
		PerSubnet24PPS:        binary.LittleEndian.Uint32(data[28:32]),
		SynCookieThreshold:    binary.LittleEndian.Uint32(data[32:36]),
		SynCookieActive:       data[36],
		ConnTrackerEnabled:    data[37],
		GeoIPEnabled:          data[38],
		PadSC:                 data[39],
		BotnetNewIPThreshold:  binary.LittleEndian.Uint32(data[40:44]),
		BotnetCooldownSeconds: binary.LittleEndian.Uint32(data[44:48]),
		BotnetModeActive:      data[48],
		PadBN:                 [3]uint8{data[49], data[50], data[51]},
	}

	return cfg, nil
}

// writeConfigToMap writes the serialized config bytes to the kiro_config BPF map.
func (cs *ConfigSync) writeConfigToMap(data []byte) error {
	keyHex := "0x00 0x00 0x00 0x00"
	valueHex := bytesToHex(data)
	return bpftoolMapUpdate(cs.configMapName, keyHex, valueHex)
}

// ReadConfig reads the current XDP config from the BPF map.
func (cs *ConfigSync) ReadConfig() (XDPConfig, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Read the full config from the BPF map using the botnet controller's
	// existing readConfigMap pattern (shared in this package).
	bc := &BotnetController{configMapName: cs.configMapName}
	data, err := bc.readConfigMap()
	if err != nil {
		return XDPConfig{}, fmt.Errorf("config_sync: read config: %w", err)
	}

	return DeserializeXDPConfig(data)
}

// LastSyncTime returns the time of the last successful config sync.
func (cs *ConfigSync) LastSyncTime() time.Time {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.lastSync
}

// ─── SYN Cookie Key Rotation ─────────────────────────────────────────────────

// GenerateSynCookieKey generates a new random SipHash key (k0, k1 as uint64)
// using crypto/rand for cryptographic security.
func GenerateSynCookieKey() (synCookieKey, error) {
	var key synCookieKey
	var buf [16]byte

	if _, err := rand.Read(buf[:]); err != nil {
		return key, fmt.Errorf("generate syn cookie key: %w", err)
	}

	key.K0 = binary.LittleEndian.Uint64(buf[0:8])
	key.K1 = binary.LittleEndian.Uint64(buf[8:16])
	return key, nil
}

// RotateSynCookieKey generates a new random SipHash key and writes it to
// the syn_cookie_key_map BPF array map at index 0.
//
// The key is used by the XDP program for SYN cookie computation (SipHash-2-4).
// Key format matches C struct syn_cookie_key: { __u64 k0; __u64 k1; } (16 bytes LE).
func (cs *ConfigSync) RotateSynCookieKey() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	key, err := GenerateSynCookieKey()
	if err != nil {
		return fmt.Errorf("config_sync: rotate key: %w", err)
	}

	if err := cs.writeSynCookieKey(key); err != nil {
		return fmt.Errorf("config_sync: write key to map: %w", err)
	}

	cs.lastKey = key
	cs.keyRotated = true
	log.Printf("config_sync: SYN cookie key rotated successfully")
	return nil
}

// writeSynCookieKey writes the SipHash key to the syn_cookie_key_map BPF map.
// Map layout: ARRAY, index 0, value = { k0(8B LE), k1(8B LE) } = 16 bytes.
func (cs *ConfigSync) writeSynCookieKey(key synCookieKey) error {
	keyHex := "0x00 0x00 0x00 0x00" // array index 0

	// Serialize the SipHash key: k0 (8 bytes LE) + k1 (8 bytes LE)
	buf := make([]byte, 16)
	binary.LittleEndian.PutUint64(buf[0:8], key.K0)
	binary.LittleEndian.PutUint64(buf[8:16], key.K1)

	valueHex := bytesToHex(buf)
	return bpftoolMapUpdate(cs.synCookieKeyMapName, keyHex, valueHex)
}

// StartKeyRotation starts a background goroutine that rotates the SYN cookie
// key at the configured interval (default: every 24 hours). The goroutine
// performs an initial key rotation immediately on start, then rotates
// periodically. It exits when the context is cancelled.
//
// Requirements: 12.1, 12.5 (key rotation for SYN cookie security)
func (cs *ConfigSync) StartKeyRotation(ctx context.Context) {
	go func() {
		// Perform initial key rotation immediately
		if err := cs.RotateSynCookieKey(); err != nil {
			log.Printf("config_sync: initial key rotation failed: %v", err)
		}

		ticker := time.NewTicker(cs.keyRotationInterval)
		defer ticker.Stop()

		log.Printf("config_sync: SYN cookie key rotation started (interval: %s)", cs.keyRotationInterval)

		for {
			select {
			case <-ctx.Done():
				log.Printf("config_sync: key rotation stopped")
				return
			case <-ticker.C:
				if err := cs.RotateSynCookieKey(); err != nil {
					log.Printf("config_sync: key rotation failed: %v", err)
				}
			}
		}
	}()
}

// KeyRotated returns whether the SYN cookie key has been rotated at least once.
func (cs *ConfigSync) KeyRotated() bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.keyRotated
}
