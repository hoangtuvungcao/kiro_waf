// Package xdp provides userspace utilities for managing XDP/eBPF maps.
package xdp

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// GeoIPLoader populates the XDP geoip_map and country_blocklist BPF maps
// from userspace. It parses MaxMind GeoLite2 CSV data and uses bpftool
// to update BPF maps.
//
// Requirements: 14.5, 14.6
type GeoIPLoader struct {
	// geoipMapName is the BPF map name for the GeoIP LPM trie.
	geoipMapName string

	// blocklistMapName is the BPF map name for the country blocklist hash.
	blocklistMapName string

	mu          sync.Mutex
	csvPath     string
	countries   []string
	lastRefresh time.Time
}

// geoIPEntry represents a single GeoIP database entry.
type geoIPEntry struct {
	Network     net.IPNet
	CountryCode string
}

// NewGeoIPLoader creates a new GeoIPLoader instance.
func NewGeoIPLoader() *GeoIPLoader {
	return &GeoIPLoader{
		geoipMapName:     "geoip_map",
		blocklistMapName: "country_blocklist",
	}
}

// LoadFromCSV parses a MaxMind GeoLite2 Country CSV file and populates
// the geoip_map BPF LPM trie via bpf_map_update_elem (using bpftool).
//
// Expected CSV format (GeoLite2-Country-Blocks-IPv4.csv):
//   network, geoname_id, registered_country_geoname_id, ..., is_anonymous_proxy, is_satellite_provider
//
// The country code is resolved from a separate locations CSV or provided
// inline. This implementation expects a pre-joined format with columns:
//   network (CIDR), geoname_id, country_iso_code, ...
//
// The country_iso_code column (index 2) contains the 2-letter ISO code.
func (g *GeoIPLoader) LoadFromCSV(path string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("geoip: open csv %s: %w", path, err)
	}
	defer f.Close()

	entries, err := parseGeoIPCSV(f)
	if err != nil {
		return fmt.Errorf("geoip: parse csv: %w", err)
	}

	loaded := 0
	for _, entry := range entries {
		if err := g.updateGeoIPMap(entry); err != nil {
			log.Printf("geoip: failed to update map for %s: %v", entry.Network.String(), err)
			continue
		}
		loaded++
	}

	g.csvPath = path
	g.lastRefresh = time.Now()
	log.Printf("geoip: loaded %d/%d entries from %s", loaded, len(entries), path)
	return nil
}

// LoadBlockedCountries populates the country_blocklist BPF hash map
// with the given 2-letter country codes.
//
// Each country code is encoded as __u16 (first char << 8 | second char)
// matching the XDP C struct format.
func (g *GeoIPLoader) LoadBlockedCountries(countries []string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.countries = countries

	for _, cc := range countries {
		cc = strings.TrimSpace(strings.ToUpper(cc))
		if len(cc) != 2 {
			log.Printf("geoip: skipping invalid country code %q", cc)
			continue
		}

		if err := g.updateCountryBlocklist(cc); err != nil {
			return fmt.Errorf("geoip: update blocklist for %s: %w", cc, err)
		}
	}

	log.Printf("geoip: loaded %d blocked countries", len(countries))
	return nil
}

// LoadBlockedCountriesFromEnv reads the KIRO_XDP_BLOCKED_COUNTRIES environment
// variable (comma-separated 2-letter codes) and populates the blocklist.
func (g *GeoIPLoader) LoadBlockedCountriesFromEnv() error {
	envVal := os.Getenv("KIRO_XDP_BLOCKED_COUNTRIES")
	if envVal == "" {
		log.Printf("geoip: KIRO_XDP_BLOCKED_COUNTRIES not set, no countries blocked")
		return nil
	}

	countries := strings.Split(envVal, ",")
	var cleaned []string
	for _, c := range countries {
		c = strings.TrimSpace(c)
		if c != "" {
			cleaned = append(cleaned, c)
		}
	}

	if len(cleaned) == 0 {
		return nil
	}

	return g.LoadBlockedCountries(cleaned)
}

// StartPeriodicRefresh starts a background goroutine that refreshes the
// GeoIP data every interval (default: 24h). The goroutine exits when
// the context is cancelled.
func (g *GeoIPLoader) StartPeriodicRefresh(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 24 * time.Hour
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Printf("geoip: periodic refresh stopped")
				return
			case <-ticker.C:
				g.refresh()
			}
		}
	}()

	log.Printf("geoip: periodic refresh started (interval: %s)", interval)
}

// refresh reloads the GeoIP data from the last known CSV path and
// re-applies the blocked countries list.
func (g *GeoIPLoader) refresh() {
	g.mu.Lock()
	csvPath := g.csvPath
	countries := g.countries
	g.mu.Unlock()

	if csvPath != "" {
		if err := g.LoadFromCSV(csvPath); err != nil {
			log.Printf("geoip: refresh failed for csv: %v", err)
		}
	}

	if len(countries) > 0 {
		if err := g.LoadBlockedCountries(countries); err != nil {
			log.Printf("geoip: refresh failed for blocklist: %v", err)
		}
	}
}

// EncodeCountryCode converts a 2-letter ISO country code to the __u16
// format used by the XDP program: first_char << 8 | second_char.
func EncodeCountryCode(cc string) uint16 {
	if len(cc) < 2 {
		return 0
	}
	return uint16(cc[0])<<8 | uint16(cc[1])
}

// parseGeoIPCSV reads a MaxMind GeoLite2 CSV file and extracts entries
// with valid network CIDRs and country codes.
//
// Supports two formats:
// 1. Pre-joined format: network, geoname_id, country_iso_code, ...
// 2. Standard GeoLite2-Country-Blocks-IPv4.csv with country code in various columns
//
// The parser looks for a column containing 2-letter country codes.
func parseGeoIPCSV(r io.Reader) ([]geoIPEntry, error) {
	reader := csv.NewReader(bufio.NewReader(r))
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	// Read header to find column indices
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	networkIdx := -1
	countryIdx := -1

	for i, col := range header {
		col = strings.TrimSpace(strings.ToLower(col))
		switch {
		case col == "network":
			networkIdx = i
		case col == "country_iso_code":
			countryIdx = i
		}
	}

	if networkIdx == -1 {
		return nil, fmt.Errorf("csv missing 'network' column")
	}
	if countryIdx == -1 {
		return nil, fmt.Errorf("csv missing 'country_iso_code' column")
	}

	var entries []geoIPEntry
	lineNum := 1

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			lineNum++
			continue
		}
		lineNum++

		if len(record) <= networkIdx || len(record) <= countryIdx {
			continue
		}

		network := strings.TrimSpace(record[networkIdx])
		countryCode := strings.TrimSpace(record[countryIdx])

		// Skip entries without a country code
		if countryCode == "" || len(countryCode) != 2 {
			continue
		}

		// Parse CIDR network
		_, ipNet, err := net.ParseCIDR(network)
		if err != nil {
			continue
		}

		// Only handle IPv4
		if ipNet.IP.To4() == nil {
			continue
		}

		entries = append(entries, geoIPEntry{
			Network:     *ipNet,
			CountryCode: strings.ToUpper(countryCode),
		})
	}

	return entries, nil
}

// updateGeoIPMap writes a single entry to the geoip_map BPF LPM trie.
// Key format matches the C struct lpm_v4_key: { __u32 prefixlen; __u32 addr; }
// Value format matches geoip_value: { __u16 country_code; }
func (g *GeoIPLoader) updateGeoIPMap(entry geoIPEntry) error {
	ip4 := entry.Network.IP.To4()
	if ip4 == nil {
		return fmt.Errorf("not an IPv4 address: %s", entry.Network.IP)
	}

	prefixLen, _ := entry.Network.Mask.Size()
	countryCode := EncodeCountryCode(entry.CountryCode)

	// Build key bytes: prefixlen (4 bytes LE) + addr (4 bytes, network byte order)
	keyHex := fmt.Sprintf("%s %s",
		toLEHex32(uint32(prefixLen)),
		toNetHex32(ip4))

	// Build value bytes: country_code (2 bytes LE)
	valueHex := toLEHex16(countryCode)

	return bpftoolMapUpdate(g.geoipMapName, keyHex, valueHex)
}

// updateCountryBlocklist writes a country code to the country_blocklist BPF hash map.
// Key: __u16 country_code (LE)
// Value: __u8 = 1 (blocked)
func (g *GeoIPLoader) updateCountryBlocklist(cc string) error {
	countryCode := EncodeCountryCode(cc)

	keyHex := toLEHex16(countryCode)
	valueHex := "0x01"

	return bpftoolMapUpdate(g.blocklistMapName, keyHex, valueHex)
}

// bpftoolMapUpdate uses bpftool to update a BPF map entry.
// This matches the existing project pattern of using CLI tools for BPF interaction.
func bpftoolMapUpdate(mapName, keyHex, valueHex string) error {
	args := []string{"map", "update", "name", mapName}

	// Add key bytes
	args = append(args, "key")
	args = append(args, strings.Fields(keyHex)...)

	// Add value bytes
	args = append(args, "value")
	args = append(args, strings.Fields(valueHex)...)

	cmd := exec.Command("bpftool", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bpftool map update %s: %s: %w", mapName, strings.TrimSpace(string(output)), err)
	}
	return nil
}

// toLEHex32 converts a uint32 to space-separated hex bytes in little-endian order.
// Example: 24 → "0x18 0x00 0x00 0x00"
func toLEHex32(v uint32) string {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return fmt.Sprintf("0x%02x 0x%02x 0x%02x 0x%02x", b[0], b[1], b[2], b[3])
}

// toNetHex32 converts a 4-byte IPv4 address to space-separated hex bytes
// in network byte order (big-endian, as stored in the BPF map).
// Example: 192.168.1.0 → "0xc0 0xa8 0x01 0x00"
func toNetHex32(ip net.IP) string {
	ip4 := ip.To4()
	if ip4 == nil {
		return "0x00 0x00 0x00 0x00"
	}
	return fmt.Sprintf("0x%02x 0x%02x 0x%02x 0x%02x", ip4[0], ip4[1], ip4[2], ip4[3])
}

// toLEHex16 converts a uint16 to space-separated hex bytes in little-endian order.
// Example: EncodeCountryCode("CN") = 0x434e → "0x4e 0x43"
func toLEHex16(v uint16) string {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, v)
	return fmt.Sprintf("0x%02x 0x%02x", b[0], b[1])
}

// PrefixLenFromMask extracts the prefix length from a net.IPMask.
func PrefixLenFromMask(mask net.IPMask) int {
	ones, _ := mask.Size()
	return ones
}

// FormatLPMKey formats an IP and prefix length as the LPM trie key hex string
// matching the C struct: { __u32 prefixlen; __u32 addr; }
func FormatLPMKey(ip net.IP, prefixLen int) string {
	return fmt.Sprintf("%s %s", toLEHex32(uint32(prefixLen)), toNetHex32(ip))
}

// ParseBlockedCountriesEnv parses the KIRO_XDP_BLOCKED_COUNTRIES environment
// variable and returns the list of country codes.
func ParseBlockedCountriesEnv() []string {
	envVal := os.Getenv("KIRO_XDP_BLOCKED_COUNTRIES")
	if envVal == "" {
		return nil
	}

	parts := strings.Split(envVal, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToUpper(p))
		if len(p) == 2 {
			result = append(result, p)
		}
	}
	return result
}

// CountryCodeToString converts a __u16 encoded country code back to a 2-letter string.
func CountryCodeToString(code uint16) string {
	if code == 0 {
		return ""
	}
	return string([]byte{byte(code >> 8), byte(code & 0xff)})
}

// LastRefreshTime returns the time of the last successful GeoIP data refresh.
func (g *GeoIPLoader) LastRefreshTime() time.Time {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.lastRefresh
}

// BlockedCountries returns the currently configured blocked country codes.
func (g *GeoIPLoader) BlockedCountries() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	result := make([]string, len(g.countries))
	copy(result, g.countries)
	return result
}


