package xdp

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func TestEncodeCountryCode(t *testing.T) {
	tests := []struct {
		input    string
		expected uint16
	}{
		{"CN", uint16('C')<<8 | uint16('N')},
		{"US", uint16('U')<<8 | uint16('S')},
		{"RU", uint16('R')<<8 | uint16('U')},
		{"KP", uint16('K')<<8 | uint16('P')},
		{"DE", uint16('D')<<8 | uint16('E')},
		{"", 0},
		{"A", 0},
	}

	for _, tt := range tests {
		got := EncodeCountryCode(tt.input)
		if got != tt.expected {
			t.Errorf("EncodeCountryCode(%q) = 0x%04x, want 0x%04x", tt.input, got, tt.expected)
		}
	}
}

func TestCountryCodeToString(t *testing.T) {
	tests := []struct {
		input    uint16
		expected string
	}{
		{uint16('C')<<8 | uint16('N'), "CN"},
		{uint16('U')<<8 | uint16('S'), "US"},
		{0, ""},
	}

	for _, tt := range tests {
		got := CountryCodeToString(tt.input)
		if got != tt.expected {
			t.Errorf("CountryCodeToString(0x%04x) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	codes := []string{"CN", "US", "RU", "KP", "DE", "FR", "JP", "BR", "AU", "ZA"}
	for _, cc := range codes {
		encoded := EncodeCountryCode(cc)
		decoded := CountryCodeToString(encoded)
		if decoded != cc {
			t.Errorf("round-trip failed for %q: encoded=0x%04x, decoded=%q", cc, encoded, decoded)
		}
	}
}

func TestToLEHex32(t *testing.T) {
	tests := []struct {
		input    uint32
		expected string
	}{
		{24, "0x18 0x00 0x00 0x00"},
		{32, "0x20 0x00 0x00 0x00"},
		{8, "0x08 0x00 0x00 0x00"},
		{0, "0x00 0x00 0x00 0x00"},
	}

	for _, tt := range tests {
		got := toLEHex32(tt.input)
		if got != tt.expected {
			t.Errorf("toLEHex32(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestToNetHex32(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"192.168.1.0", "0xc0 0xa8 0x01 0x00"},
		{"10.0.0.0", "0x0a 0x00 0x00 0x00"},
		{"1.2.3.4", "0x01 0x02 0x03 0x04"},
		{"255.255.255.0", "0xff 0xff 0xff 0x00"},
	}

	for _, tt := range tests {
		ip := net.ParseIP(tt.input)
		got := toNetHex32(ip)
		if got != tt.expected {
			t.Errorf("toNetHex32(%s) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestToLEHex16(t *testing.T) {
	// "CN" = 'C'<<8 | 'N' = 0x434e
	// In LE: 0x4e 0x43
	cn := EncodeCountryCode("CN")
	got := toLEHex16(cn)
	expected := "0x4e 0x43"
	if got != expected {
		t.Errorf("toLEHex16(CN=0x%04x) = %q, want %q", cn, got, expected)
	}

	// "US" = 'U'<<8 | 'S' = 0x5553
	// In LE: 0x53 0x55
	us := EncodeCountryCode("US")
	got = toLEHex16(us)
	expected = "0x53 0x55"
	if got != expected {
		t.Errorf("toLEHex16(US=0x%04x) = %q, want %q", us, got, expected)
	}
}

func TestFormatLPMKey(t *testing.T) {
	ip := net.ParseIP("192.168.1.0")
	got := FormatLPMKey(ip, 24)
	// prefixlen=24 in LE: 0x18 0x00 0x00 0x00
	// addr in network order: 0xc0 0xa8 0x01 0x00
	expected := "0x18 0x00 0x00 0x00 0xc0 0xa8 0x01 0x00"
	if got != expected {
		t.Errorf("FormatLPMKey(192.168.1.0, 24) = %q, want %q", got, expected)
	}
}

func TestParseGeoIPCSV(t *testing.T) {
	csvData := `network,geoname_id,country_iso_code,is_anonymous_proxy
1.0.0.0/24,2077456,AU,0
1.0.1.0/24,1814991,CN,0
1.0.4.0/22,2077456,AU,0
2.0.0.0/12,6252001,US,0
invalid_cidr,123,XX,0
10.0.0.0/8,,,,
`

	entries, err := parseGeoIPCSV(strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("parseGeoIPCSV() error: %v", err)
	}

	// Should have 4 valid entries (skip invalid CIDR and empty country code)
	if len(entries) != 4 {
		t.Fatalf("parseGeoIPCSV() got %d entries, want 4", len(entries))
	}

	// Verify first entry
	if entries[0].CountryCode != "AU" {
		t.Errorf("entries[0].CountryCode = %q, want %q", entries[0].CountryCode, "AU")
	}
	if entries[0].Network.String() != "1.0.0.0/24" {
		t.Errorf("entries[0].Network = %q, want %q", entries[0].Network.String(), "1.0.0.0/24")
	}

	// Verify CN entry
	if entries[1].CountryCode != "CN" {
		t.Errorf("entries[1].CountryCode = %q, want %q", entries[1].CountryCode, "CN")
	}

	// Verify /22 prefix
	if entries[2].Network.String() != "1.0.4.0/22" {
		t.Errorf("entries[2].Network = %q, want %q", entries[2].Network.String(), "1.0.4.0/22")
	}
}

func TestParseGeoIPCSV_MissingColumns(t *testing.T) {
	// Missing network column
	csvData := `geoname_id,country_iso_code
123,US
`
	_, err := parseGeoIPCSV(strings.NewReader(csvData))
	if err == nil {
		t.Error("expected error for missing 'network' column")
	}

	// Missing country_iso_code column
	csvData = `network,geoname_id
1.0.0.0/24,123
`
	_, err = parseGeoIPCSV(strings.NewReader(csvData))
	if err == nil {
		t.Error("expected error for missing 'country_iso_code' column")
	}
}

func TestParseGeoIPCSV_IPv6Skipped(t *testing.T) {
	csvData := `network,geoname_id,country_iso_code
2001:200::/32,1861060,JP
1.0.0.0/24,2077456,AU
`

	entries, err := parseGeoIPCSV(strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("parseGeoIPCSV() error: %v", err)
	}

	// Only IPv4 entry should be included
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1 (IPv6 should be skipped)", len(entries))
	}
	if entries[0].CountryCode != "AU" {
		t.Errorf("entries[0].CountryCode = %q, want %q", entries[0].CountryCode, "AU")
	}
}

func TestParseBlockedCountriesEnv(t *testing.T) {
	// Set env var for test
	t.Setenv("KIRO_XDP_BLOCKED_COUNTRIES", "CN,RU,KP")

	countries := ParseBlockedCountriesEnv()
	if len(countries) != 3 {
		t.Fatalf("got %d countries, want 3", len(countries))
	}
	if countries[0] != "CN" || countries[1] != "RU" || countries[2] != "KP" {
		t.Errorf("got %v, want [CN RU KP]", countries)
	}
}

func TestParseBlockedCountriesEnv_Empty(t *testing.T) {
	t.Setenv("KIRO_XDP_BLOCKED_COUNTRIES", "")

	countries := ParseBlockedCountriesEnv()
	if len(countries) != 0 {
		t.Fatalf("got %d countries, want 0", len(countries))
	}
}

func TestParseBlockedCountriesEnv_WithSpaces(t *testing.T) {
	t.Setenv("KIRO_XDP_BLOCKED_COUNTRIES", " CN , RU , KP ")

	countries := ParseBlockedCountriesEnv()
	if len(countries) != 3 {
		t.Fatalf("got %d countries, want 3", len(countries))
	}
	if countries[0] != "CN" || countries[1] != "RU" || countries[2] != "KP" {
		t.Errorf("got %v, want [CN RU KP]", countries)
	}
}

func TestParseBlockedCountriesEnv_InvalidCodes(t *testing.T) {
	t.Setenv("KIRO_XDP_BLOCKED_COUNTRIES", "CN,X,RU,ABC,KP")

	countries := ParseBlockedCountriesEnv()
	// Only 2-letter codes should be included
	if len(countries) != 3 {
		t.Fatalf("got %d countries, want 3 (invalid codes filtered)", len(countries))
	}
}

func TestNewGeoIPLoader(t *testing.T) {
	loader := NewGeoIPLoader()
	if loader == nil {
		t.Fatal("NewGeoIPLoader() returned nil")
	}
	if loader.geoipMapName != "geoip_map" {
		t.Errorf("geoipMapName = %q, want %q", loader.geoipMapName, "geoip_map")
	}
	if loader.blocklistMapName != "country_blocklist" {
		t.Errorf("blocklistMapName = %q, want %q", loader.blocklistMapName, "country_blocklist")
	}
}

func TestGeoIPLoader_BlockedCountries(t *testing.T) {
	loader := NewGeoIPLoader()
	// Directly set countries (bypass bpftool which won't be available in test)
	loader.mu.Lock()
	loader.countries = []string{"CN", "RU", "KP"}
	loader.mu.Unlock()

	got := loader.BlockedCountries()
	if len(got) != 3 {
		t.Fatalf("BlockedCountries() got %d, want 3", len(got))
	}
	if got[0] != "CN" || got[1] != "RU" || got[2] != "KP" {
		t.Errorf("BlockedCountries() = %v, want [CN RU KP]", got)
	}
}

func TestGeoIPLoader_LastRefreshTime(t *testing.T) {
	loader := NewGeoIPLoader()
	if !loader.LastRefreshTime().IsZero() {
		t.Error("LastRefreshTime() should be zero initially")
	}
}

func TestStartPeriodicRefresh_ContextCancellation(t *testing.T) {
	loader := NewGeoIPLoader()
	ctx, cancel := context.WithCancel(context.Background())

	// Start with a very short interval
	loader.StartPeriodicRefresh(ctx, 10*time.Millisecond)

	// Cancel immediately
	cancel()

	// Give goroutine time to exit
	time.Sleep(50 * time.Millisecond)
	// No assertion needed — just verify no panic/deadlock
}

func TestPrefixLenFromMask(t *testing.T) {
	tests := []struct {
		mask     net.IPMask
		expected int
	}{
		{net.CIDRMask(24, 32), 24},
		{net.CIDRMask(32, 32), 32},
		{net.CIDRMask(8, 32), 8},
		{net.CIDRMask(16, 32), 16},
	}

	for _, tt := range tests {
		got := PrefixLenFromMask(tt.mask)
		if got != tt.expected {
			t.Errorf("PrefixLenFromMask(%v) = %d, want %d", tt.mask, got, tt.expected)
		}
	}
}
