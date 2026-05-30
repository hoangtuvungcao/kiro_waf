package xdp

import (
	"testing"
)

func TestNewBotnetController_Defaults(t *testing.T) {
	bc := NewBotnetController(BotnetControllerConfig{})

	if bc.threshold != 5000 {
		t.Errorf("expected default threshold 5000, got %d", bc.threshold)
	}
	if bc.cooldownDuration.Seconds() != 30 {
		t.Errorf("expected default cooldown 30s, got %s", bc.cooldownDuration)
	}
	if bc.pollInterval.Seconds() != 1 {
		t.Errorf("expected default poll interval 1s, got %s", bc.pollInterval)
	}
	if bc.configMapName != "kiro_config" {
		t.Errorf("expected config map name 'kiro_config', got %q", bc.configMapName)
	}
	if bc.rateMapName != "new_ip_rate" {
		t.Errorf("expected rate map name 'new_ip_rate', got %q", bc.rateMapName)
	}
}

func TestNewBotnetController_CustomConfig(t *testing.T) {
	bc := NewBotnetController(BotnetControllerConfig{
		Threshold:       10000,
		CooldownSeconds: 60,
		PollInterval:    2000000000, // 2s in nanoseconds
	})

	if bc.threshold != 10000 {
		t.Errorf("expected threshold 10000, got %d", bc.threshold)
	}
	if bc.cooldownDuration.Seconds() != 60 {
		t.Errorf("expected cooldown 60s, got %s", bc.cooldownDuration)
	}
}

func TestParsePerCPUCounters_RawFormat(t *testing.T) {
	// Simulate bpftool percpu output in raw "cpuN:" format
	// new_ip_counter struct: { window_start_ns(8B LE), count(4B LE), _pad(4B) }
	// cpu0: count = 1000 (0x03E8), cpu1: count = 500 (0x01F4)
	output := `cpu0: 00 00 00 00 00 00 00 00 e8 03 00 00 00 00 00 00
cpu1: 00 00 00 00 00 00 00 00 f4 01 00 00 00 00 00 00
cpu2: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
cpu3: 00 00 00 00 00 00 00 00 2c 01 00 00 00 00 00 00`

	total, err := parsePerCPUCounters(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1000 + 500 + 0 + 300 = 1800
	expected := uint32(1800)
	if total != expected {
		t.Errorf("expected total %d, got %d", expected, total)
	}
}

func TestParsePerCPUCounters_HexPrefixFormat(t *testing.T) {
	// Simulate bpftool percpu output with 0x-prefixed hex bytes
	output := `cpu0: 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0xe8 0x03 0x00 0x00 0x00 0x00 0x00 0x00
cpu1: 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0xf4 0x01 0x00 0x00 0x00 0x00 0x00 0x00`

	total, err := parsePerCPUCounters(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1000 + 500 = 1500
	expected := uint32(1500)
	if total != expected {
		t.Errorf("expected total %d, got %d", expected, total)
	}
}

func TestParsePerCPUCounters_EmptyOutput(t *testing.T) {
	total, err := parsePerCPUCounters("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 {
		t.Errorf("expected 0 for empty output, got %d", total)
	}
}

func TestParsePerCPUCounters_AllZeros(t *testing.T) {
	output := `cpu0: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
cpu1: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00`

	total, err := parsePerCPUCounters(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 {
		t.Errorf("expected 0, got %d", total)
	}
}

func TestExtractCountFromHex(t *testing.T) {
	tests := []struct {
		name     string
		hexStr   string
		expected uint32
		wantErr  bool
	}{
		{
			name:     "count 1000 at offset 8",
			hexStr:   "00 00 00 00 00 00 00 00 e8 03 00 00 00 00 00 00",
			expected: 1000,
		},
		{
			name:     "count 5000 at offset 8",
			hexStr:   "00 00 00 00 00 00 00 00 88 13 00 00 00 00 00 00",
			expected: 5000,
		},
		{
			name:     "count 0",
			hexStr:   "00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00",
			expected: 0,
		},
		{
			name:     "with 0x prefix",
			hexStr:   "0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0xe8 0x03 0x00 0x00 0x00 0x00 0x00 0x00",
			expected: 1000,
		},
		{
			name:    "too short",
			hexStr:  "00 00 00 00",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := extractCountFromHex(tt.hexStr)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if count != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, count)
			}
		})
	}
}

func TestBytesToHex(t *testing.T) {
	data := []byte{0x00, 0x01, 0xff, 0x10}
	result := bytesToHex(data)
	expected := "0x00 0x01 0xff 0x10"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestParseHexString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []byte
	}{
		{
			name:     "plain hex",
			input:    "00 01 ff 10",
			expected: []byte{0x00, 0x01, 0xff, 0x10},
		},
		{
			name:     "0x prefixed",
			input:    "0x00 0x01 0xff 0x10",
			expected: []byte{0x00, 0x01, 0xff, 0x10},
		},
		{
			name:     "empty",
			input:    "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := parseHexString(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d bytes, got %d", len(tt.expected), len(result))
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("byte %d: expected 0x%02x, got 0x%02x", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

func TestBotnetController_IsBotnetActive(t *testing.T) {
	bc := NewBotnetController(BotnetControllerConfig{Threshold: 5000})

	if bc.IsBotnetActive() {
		t.Error("expected botnet mode to be inactive initially")
	}
}

func TestBotnetController_LastRate(t *testing.T) {
	bc := NewBotnetController(BotnetControllerConfig{Threshold: 5000})

	if bc.LastRate() != 0 {
		t.Errorf("expected initial rate 0, got %d", bc.LastRate())
	}
}
