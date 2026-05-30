package xdp

import (
	"encoding/binary"
	"testing"
)

func TestSerializeXDPConfig_RoundTrip(t *testing.T) {
	cfg := XDPConfig{
		WindowNS:              1000000000, // 1 second
		PerIPPPS:              10000,
		SynPerIPPerSecond:     100,
		UDPPerIPPerSecond:     200,
		ICMPPerIPPerSecond:    50,
		DropPrivateSourceIP:   1,
		DropMalformed:         1,
		RateLimitEnabled:      1,
		DropFragments:         0,
		PerSubnet24PPS:        50000,
		SynCookieThreshold:    10000,
		SynCookieActive:       1,
		ConnTrackerEnabled:    1,
		GeoIPEnabled:          1,
		PadSC:                 0,
		BotnetNewIPThreshold:  5000,
		BotnetCooldownSeconds: 30,
		BotnetModeActive:      0,
		PadBN:                 [3]uint8{0, 0, 0},
	}

	data := SerializeXDPConfig(cfg)

	if len(data) != 52 {
		t.Fatalf("expected 52 bytes, got %d", len(data))
	}

	// Deserialize and compare
	got, err := DeserializeXDPConfig(data)
	if err != nil {
		t.Fatalf("deserialize failed: %v", err)
	}

	if got.WindowNS != cfg.WindowNS {
		t.Errorf("WindowNS: expected %d, got %d", cfg.WindowNS, got.WindowNS)
	}
	if got.PerIPPPS != cfg.PerIPPPS {
		t.Errorf("PerIPPPS: expected %d, got %d", cfg.PerIPPPS, got.PerIPPPS)
	}
	if got.SynPerIPPerSecond != cfg.SynPerIPPerSecond {
		t.Errorf("SynPerIPPerSecond: expected %d, got %d", cfg.SynPerIPPerSecond, got.SynPerIPPerSecond)
	}
	if got.UDPPerIPPerSecond != cfg.UDPPerIPPerSecond {
		t.Errorf("UDPPerIPPerSecond: expected %d, got %d", cfg.UDPPerIPPerSecond, got.UDPPerIPPerSecond)
	}
	if got.ICMPPerIPPerSecond != cfg.ICMPPerIPPerSecond {
		t.Errorf("ICMPPerIPPerSecond: expected %d, got %d", cfg.ICMPPerIPPerSecond, got.ICMPPerIPPerSecond)
	}
	if got.DropPrivateSourceIP != cfg.DropPrivateSourceIP {
		t.Errorf("DropPrivateSourceIP: expected %d, got %d", cfg.DropPrivateSourceIP, got.DropPrivateSourceIP)
	}
	if got.DropMalformed != cfg.DropMalformed {
		t.Errorf("DropMalformed: expected %d, got %d", cfg.DropMalformed, got.DropMalformed)
	}
	if got.RateLimitEnabled != cfg.RateLimitEnabled {
		t.Errorf("RateLimitEnabled: expected %d, got %d", cfg.RateLimitEnabled, got.RateLimitEnabled)
	}
	if got.DropFragments != cfg.DropFragments {
		t.Errorf("DropFragments: expected %d, got %d", cfg.DropFragments, got.DropFragments)
	}
	if got.PerSubnet24PPS != cfg.PerSubnet24PPS {
		t.Errorf("PerSubnet24PPS: expected %d, got %d", cfg.PerSubnet24PPS, got.PerSubnet24PPS)
	}
	if got.SynCookieThreshold != cfg.SynCookieThreshold {
		t.Errorf("SynCookieThreshold: expected %d, got %d", cfg.SynCookieThreshold, got.SynCookieThreshold)
	}
	if got.SynCookieActive != cfg.SynCookieActive {
		t.Errorf("SynCookieActive: expected %d, got %d", cfg.SynCookieActive, got.SynCookieActive)
	}
	if got.ConnTrackerEnabled != cfg.ConnTrackerEnabled {
		t.Errorf("ConnTrackerEnabled: expected %d, got %d", cfg.ConnTrackerEnabled, got.ConnTrackerEnabled)
	}
	if got.GeoIPEnabled != cfg.GeoIPEnabled {
		t.Errorf("GeoIPEnabled: expected %d, got %d", cfg.GeoIPEnabled, got.GeoIPEnabled)
	}
	if got.BotnetNewIPThreshold != cfg.BotnetNewIPThreshold {
		t.Errorf("BotnetNewIPThreshold: expected %d, got %d", cfg.BotnetNewIPThreshold, got.BotnetNewIPThreshold)
	}
	if got.BotnetCooldownSeconds != cfg.BotnetCooldownSeconds {
		t.Errorf("BotnetCooldownSeconds: expected %d, got %d", cfg.BotnetCooldownSeconds, got.BotnetCooldownSeconds)
	}
	if got.BotnetModeActive != cfg.BotnetModeActive {
		t.Errorf("BotnetModeActive: expected %d, got %d", cfg.BotnetModeActive, got.BotnetModeActive)
	}
}

func TestSerializeXDPConfig_FieldOffsets(t *testing.T) {
	// Verify specific field offsets match the C struct layout
	cfg := XDPConfig{
		WindowNS:              0x0102030405060708,
		PerIPPPS:              0x11121314,
		SynPerIPPerSecond:     0x21222324,
		UDPPerIPPerSecond:     0x31323334,
		ICMPPerIPPerSecond:    0x41424344,
		DropPrivateSourceIP:   0xA1,
		DropMalformed:         0xA2,
		RateLimitEnabled:      0xA3,
		DropFragments:         0xA4,
		PerSubnet24PPS:        0x51525354,
		SynCookieThreshold:    0x61626364,
		SynCookieActive:       0xB1,
		ConnTrackerEnabled:    0xB2,
		GeoIPEnabled:          0xB3,
		PadSC:                 0xB4,
		BotnetNewIPThreshold:  0x71727374,
		BotnetCooldownSeconds: 0x81828384,
		BotnetModeActive:      0xC1,
		PadBN:                 [3]uint8{0xC2, 0xC3, 0xC4},
	}

	data := SerializeXDPConfig(cfg)

	// Check offset 0: window_ns
	if binary.LittleEndian.Uint64(data[0:8]) != 0x0102030405060708 {
		t.Errorf("offset 0 (window_ns): got 0x%x", binary.LittleEndian.Uint64(data[0:8]))
	}
	// Check offset 8: per_ip_pps
	if binary.LittleEndian.Uint32(data[8:12]) != 0x11121314 {
		t.Errorf("offset 8 (per_ip_pps): got 0x%x", binary.LittleEndian.Uint32(data[8:12]))
	}
	// Check offset 12: syn_per_ip_per_second
	if binary.LittleEndian.Uint32(data[12:16]) != 0x21222324 {
		t.Errorf("offset 12 (syn_per_ip_per_second): got 0x%x", binary.LittleEndian.Uint32(data[12:16]))
	}
	// Check offset 16: udp_per_ip_per_second
	if binary.LittleEndian.Uint32(data[16:20]) != 0x31323334 {
		t.Errorf("offset 16 (udp_per_ip_per_second): got 0x%x", binary.LittleEndian.Uint32(data[16:20]))
	}
	// Check offset 20: icmp_per_ip_per_second
	if binary.LittleEndian.Uint32(data[20:24]) != 0x41424344 {
		t.Errorf("offset 20 (icmp_per_ip_per_second): got 0x%x", binary.LittleEndian.Uint32(data[20:24]))
	}
	// Check offset 24-27: uint8 fields
	if data[24] != 0xA1 {
		t.Errorf("offset 24 (drop_private_source_ip): got 0x%x", data[24])
	}
	if data[25] != 0xA2 {
		t.Errorf("offset 25 (drop_malformed): got 0x%x", data[25])
	}
	if data[26] != 0xA3 {
		t.Errorf("offset 26 (rate_limit_enabled): got 0x%x", data[26])
	}
	if data[27] != 0xA4 {
		t.Errorf("offset 27 (drop_fragments): got 0x%x", data[27])
	}
	// Check offset 28: per_subnet24_pps
	if binary.LittleEndian.Uint32(data[28:32]) != 0x51525354 {
		t.Errorf("offset 28 (per_subnet24_pps): got 0x%x", binary.LittleEndian.Uint32(data[28:32]))
	}
	// Check offset 32: syn_cookie_threshold
	if binary.LittleEndian.Uint32(data[32:36]) != 0x61626364 {
		t.Errorf("offset 32 (syn_cookie_threshold): got 0x%x", binary.LittleEndian.Uint32(data[32:36]))
	}
	// Check offset 36: syn_cookie_active
	if data[36] != 0xB1 {
		t.Errorf("offset 36 (syn_cookie_active): got 0x%x", data[36])
	}
	// Check offset 37: conn_tracker_enabled
	if data[37] != 0xB2 {
		t.Errorf("offset 37 (conn_tracker_enabled): got 0x%x", data[37])
	}
	// Check offset 38: geoip_enabled
	if data[38] != 0xB3 {
		t.Errorf("offset 38 (geoip_enabled): got 0x%x", data[38])
	}
	// Check offset 39: _pad_sc
	if data[39] != 0xB4 {
		t.Errorf("offset 39 (_pad_sc): got 0x%x", data[39])
	}
	// Check offset 40: botnet_new_ip_threshold
	if binary.LittleEndian.Uint32(data[40:44]) != 0x71727374 {
		t.Errorf("offset 40 (botnet_new_ip_threshold): got 0x%x", binary.LittleEndian.Uint32(data[40:44]))
	}
	// Check offset 44: botnet_cooldown_seconds
	if binary.LittleEndian.Uint32(data[44:48]) != 0x81828384 {
		t.Errorf("offset 44 (botnet_cooldown_seconds): got 0x%x", binary.LittleEndian.Uint32(data[44:48]))
	}
	// Check offset 48: botnet_mode_active
	if data[48] != 0xC1 {
		t.Errorf("offset 48 (botnet_mode_active): got 0x%x", data[48])
	}
	// Check offset 49-51: _pad_bn
	if data[49] != 0xC2 || data[50] != 0xC3 || data[51] != 0xC4 {
		t.Errorf("offset 49-51 (_pad_bn): got 0x%x 0x%x 0x%x", data[49], data[50], data[51])
	}
}

func TestSerializeXDPConfig_ZeroValues(t *testing.T) {
	cfg := XDPConfig{}
	data := SerializeXDPConfig(cfg)

	if len(data) != 52 {
		t.Fatalf("expected 52 bytes, got %d", len(data))
	}

	// All bytes should be zero
	for i, b := range data {
		if b != 0 {
			t.Errorf("byte %d: expected 0x00, got 0x%02x", i, b)
		}
	}
}

func TestDeserializeXDPConfig_TooShort(t *testing.T) {
	data := make([]byte, 10) // too short
	_, err := DeserializeXDPConfig(data)
	if err == nil {
		t.Error("expected error for short data, got nil")
	}
}

func TestDeserializeXDPConfig_ExactSize(t *testing.T) {
	data := make([]byte, 52)
	// Set some known values
	binary.LittleEndian.PutUint32(data[32:36], 10000) // syn_cookie_threshold
	data[36] = 1                                       // syn_cookie_active
	data[37] = 1                                       // conn_tracker_enabled
	data[38] = 1                                       // geoip_enabled

	cfg, err := DeserializeXDPConfig(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SynCookieThreshold != 10000 {
		t.Errorf("SynCookieThreshold: expected 10000, got %d", cfg.SynCookieThreshold)
	}
	if cfg.SynCookieActive != 1 {
		t.Errorf("SynCookieActive: expected 1, got %d", cfg.SynCookieActive)
	}
	if cfg.ConnTrackerEnabled != 1 {
		t.Errorf("ConnTrackerEnabled: expected 1, got %d", cfg.ConnTrackerEnabled)
	}
	if cfg.GeoIPEnabled != 1 {
		t.Errorf("GeoIPEnabled: expected 1, got %d", cfg.GeoIPEnabled)
	}
}

func TestGenerateSynCookieKey(t *testing.T) {
	key1, err := GenerateSynCookieKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	key2, err := GenerateSynCookieKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Two random keys should be different (probability of collision is negligible)
	if key1.K0 == key2.K0 && key1.K1 == key2.K1 {
		t.Error("two generated keys should not be identical")
	}

	// Keys should not be all zeros (extremely unlikely with crypto/rand)
	if key1.K0 == 0 && key1.K1 == 0 {
		t.Error("generated key should not be all zeros")
	}
}

func TestGenerateSynCookieKey_NonZero(t *testing.T) {
	// Generate multiple keys and verify they have non-zero components
	for i := 0; i < 10; i++ {
		key, err := GenerateSynCookieKey()
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		// At least one of K0 or K1 should be non-zero
		if key.K0 == 0 && key.K1 == 0 {
			t.Errorf("iteration %d: both K0 and K1 are zero", i)
		}
	}
}

func TestNewConfigSync_Defaults(t *testing.T) {
	cs := NewConfigSync(ConfigSyncOptions{})

	if cs.configMapName != "kiro_config" {
		t.Errorf("expected config map name 'kiro_config', got %q", cs.configMapName)
	}
	if cs.synCookieKeyMapName != "syn_cookie_key_map" {
		t.Errorf("expected syn cookie key map name 'syn_cookie_key_map', got %q", cs.synCookieKeyMapName)
	}
	if cs.keyRotationInterval != 24*60*60*1000000000 { // 24h in nanoseconds
		t.Errorf("expected default key rotation interval 24h, got %s", cs.keyRotationInterval)
	}
	if cs.keyRotated {
		t.Error("expected keyRotated to be false initially")
	}
}

func TestNewConfigSync_CustomInterval(t *testing.T) {
	cs := NewConfigSync(ConfigSyncOptions{
		KeyRotationInterval: 12 * 60 * 60 * 1000000000, // 12h
	})

	if cs.keyRotationInterval != 12*60*60*1000000000 {
		t.Errorf("expected custom key rotation interval 12h, got %s", cs.keyRotationInterval)
	}
}

func TestConfigSync_KeyRotated_InitiallyFalse(t *testing.T) {
	cs := NewConfigSync(ConfigSyncOptions{})
	if cs.KeyRotated() {
		t.Error("expected KeyRotated() to be false initially")
	}
}

func TestSerializeXDPConfig_NewFieldsPresent(t *testing.T) {
	// Verify that the new fields (syn_cookie_threshold, syn_cookie_active,
	// conn_tracker_enabled, geoip_enabled, botnet_*) are correctly serialized
	cfg := XDPConfig{
		SynCookieThreshold:    10000,
		SynCookieActive:       1,
		ConnTrackerEnabled:    1,
		GeoIPEnabled:          1,
		BotnetNewIPThreshold:  5000,
		BotnetCooldownSeconds: 30,
		BotnetModeActive:      1,
	}

	data := SerializeXDPConfig(cfg)

	// Verify syn_cookie_threshold at offset 32
	if binary.LittleEndian.Uint32(data[32:36]) != 10000 {
		t.Errorf("syn_cookie_threshold: expected 10000, got %d", binary.LittleEndian.Uint32(data[32:36]))
	}
	// Verify syn_cookie_active at offset 36
	if data[36] != 1 {
		t.Errorf("syn_cookie_active: expected 1, got %d", data[36])
	}
	// Verify conn_tracker_enabled at offset 37
	if data[37] != 1 {
		t.Errorf("conn_tracker_enabled: expected 1, got %d", data[37])
	}
	// Verify geoip_enabled at offset 38
	if data[38] != 1 {
		t.Errorf("geoip_enabled: expected 1, got %d", data[38])
	}
	// Verify botnet_new_ip_threshold at offset 40
	if binary.LittleEndian.Uint32(data[40:44]) != 5000 {
		t.Errorf("botnet_new_ip_threshold: expected 5000, got %d", binary.LittleEndian.Uint32(data[40:44]))
	}
	// Verify botnet_cooldown_seconds at offset 44
	if binary.LittleEndian.Uint32(data[44:48]) != 30 {
		t.Errorf("botnet_cooldown_seconds: expected 30, got %d", binary.LittleEndian.Uint32(data[44:48]))
	}
	// Verify botnet_mode_active at offset 48
	if data[48] != 1 {
		t.Errorf("botnet_mode_active: expected 1, got %d", data[48])
	}
}
