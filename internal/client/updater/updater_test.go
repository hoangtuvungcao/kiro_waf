package updater

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClampPollInterval(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Duration
		expected time.Duration
	}{
		{
			name:     "zero returns default",
			input:    0,
			expected: DefaultPollInterval,
		},
		{
			name:     "negative returns default",
			input:    -10 * time.Second,
			expected: DefaultPollInterval,
		},
		{
			name:     "below minimum clamps to minimum",
			input:    30 * time.Second,
			expected: MinPollInterval,
		},
		{
			name:     "exactly minimum stays at minimum",
			input:    60 * time.Second,
			expected: MinPollInterval,
		},
		{
			name:     "within range stays unchanged",
			input:    300 * time.Second,
			expected: 300 * time.Second,
		},
		{
			name:     "exactly maximum stays at maximum",
			input:    86400 * time.Second,
			expected: MaxPollInterval,
		},
		{
			name:     "above maximum clamps to maximum",
			input:    100000 * time.Second,
			expected: MaxPollInterval,
		},
		{
			name:     "1 second below minimum clamps to minimum",
			input:    59 * time.Second,
			expected: MinPollInterval,
		},
		{
			name:     "1 second above maximum clamps to maximum",
			input:    86401 * time.Second,
			expected: MaxPollInterval,
		},
		{
			name:     "mid-range value stays unchanged",
			input:    3600 * time.Second,
			expected: 3600 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClampPollInterval(tt.input)
			if result != tt.expected {
				t.Errorf("ClampPollInterval(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNewOTAUpdater_ClampsPollInterval(t *testing.T) {
	tests := []struct {
		name     string
		interval time.Duration
		expected time.Duration
	}{
		{"zero gets default", 0, DefaultPollInterval},
		{"below min gets clamped", 10 * time.Second, MinPollInterval},
		{"above max gets clamped", 100000 * time.Second, MaxPollInterval},
		{"valid stays unchanged", 600 * time.Second, 600 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := NewOTAUpdater(UpdaterConfig{
				MasterURL:      "http://localhost",
				LicenseKey:     "test-key",
				Component:      "kiro-client-waf",
				Channel:        "stable",
				CurrentVersion: "1.0.0",
				PollInterval:   tt.interval,
			})
			if u.config.PollInterval != tt.expected {
				t.Errorf("NewOTAUpdater with PollInterval=%v: got config.PollInterval=%v, want %v",
					tt.interval, u.config.PollInterval, tt.expected)
			}
		})
	}
}

func TestNewOTAUpdater_DefaultTimeouts(t *testing.T) {
	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:      "http://localhost",
		LicenseKey:     "test-key",
		Component:      "kiro-client-waf",
		Channel:        "stable",
		CurrentVersion: "1.0.0",
	})

	if u.config.DownloadTimeout != DefaultDownloadTimeout {
		t.Errorf("DownloadTimeout = %v, want %v", u.config.DownloadTimeout, DefaultDownloadTimeout)
	}
	if u.config.HealthTimeout != DefaultHealthTimeout {
		t.Errorf("HealthTimeout = %v, want %v", u.config.HealthTimeout, DefaultHealthTimeout)
	}
}

func TestOTAUpdater_CheckForUpdate_NoUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/update/check" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Header.Get("X-License-Key") != "test-key" {
			t.Errorf("missing or wrong license key header")
		}

		resp := updateCheckResponse{
			UpdateAvailable: false,
			Release:         nil,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:      server.URL,
		LicenseKey:     "test-key",
		Component:      "kiro-client-waf",
		Channel:        "stable",
		CurrentVersion: "1.0.0",
		PollInterval:   300 * time.Second,
	})

	info, err := u.CheckForUpdate(context.Background())
	if err != nil {
		t.Fatalf("CheckForUpdate returned error: %v", err)
	}
	if info != nil {
		t.Errorf("expected nil UpdateInfo when no update available, got %+v", info)
	}
}

func TestOTAUpdater_CheckForUpdate_UpdateAvailable(t *testing.T) {
	expectedInfo := &UpdateInfo{
		Version:     "1.1.0",
		ArtifactURL: "https://firewall.vpsgen.com/api/v1/download/client-waf",
		SHA256:      "abc123def456",
		Notes:       "Bug fixes and improvements",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := updateCheckResponse{
			UpdateAvailable: true,
			Release:         expectedInfo,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:      server.URL,
		LicenseKey:     "test-key",
		Component:      "kiro-client-waf",
		Channel:        "stable",
		CurrentVersion: "1.0.0",
		PollInterval:   300 * time.Second,
	})

	info, err := u.CheckForUpdate(context.Background())
	if err != nil {
		t.Fatalf("CheckForUpdate returned error: %v", err)
	}
	if info == nil {
		t.Fatal("expected UpdateInfo, got nil")
	}
	if info.Version != expectedInfo.Version {
		t.Errorf("Version = %q, want %q", info.Version, expectedInfo.Version)
	}
	if info.ArtifactURL != expectedInfo.ArtifactURL {
		t.Errorf("ArtifactURL = %q, want %q", info.ArtifactURL, expectedInfo.ArtifactURL)
	}
	if info.SHA256 != expectedInfo.SHA256 {
		t.Errorf("SHA256 = %q, want %q", info.SHA256, expectedInfo.SHA256)
	}
}

func TestOTAUpdater_CheckForUpdate_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:      server.URL,
		LicenseKey:     "test-key",
		Component:      "kiro-client-waf",
		Channel:        "stable",
		CurrentVersion: "1.0.0",
		PollInterval:   300 * time.Second,
	})

	_, err := u.CheckForUpdate(context.Background())
	if err == nil {
		t.Fatal("expected error for server error response, got nil")
	}
}

func TestOTAUpdater_CheckForUpdate_Unreachable(t *testing.T) {
	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:      "http://127.0.0.1:1", // unreachable port
		LicenseKey:     "test-key",
		Component:      "kiro-client-waf",
		Channel:        "stable",
		CurrentVersion: "1.0.0",
		PollInterval:   300 * time.Second,
	})

	_, err := u.CheckForUpdate(context.Background())
	if err == nil {
		t.Fatal("expected error for unreachable server, got nil")
	}
}

func TestOTAUpdater_TriggerUpdateCheck(t *testing.T) {
	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:      "http://localhost",
		LicenseKey:     "test-key",
		Component:      "kiro-client-waf",
		Channel:        "stable",
		CurrentVersion: "1.0.0",
		PollInterval:   300 * time.Second,
	})

	// Trigger should not block
	u.TriggerUpdateCheck()

	// Second trigger should also not block (dropped since one is pending)
	u.TriggerUpdateCheck()

	// Verify signal is in channel
	select {
	case <-u.pushCh:
		// Good, signal received
	default:
		t.Error("expected signal in pushCh after TriggerUpdateCheck")
	}

	// Channel should be empty now
	select {
	case <-u.pushCh:
		t.Error("expected empty pushCh after consuming signal")
	default:
		// Good, empty
	}
}

func TestOTAUpdater_State(t *testing.T) {
	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:      "http://localhost",
		LicenseKey:     "test-key",
		Component:      "kiro-client-waf",
		Channel:        "stable",
		CurrentVersion: "1.0.0",
		PollInterval:   300 * time.Second,
		BackupPath:     "/usr/local/bin/kiro-client-waf.prev",
	})

	state := u.State()
	if state.CurrentVersion != "1.0.0" {
		t.Errorf("CurrentVersion = %q, want %q", state.CurrentVersion, "1.0.0")
	}
	if state.Status != "idle" {
		t.Errorf("Status = %q, want %q", state.Status, "idle")
	}
	if state.BackupPath != "/usr/local/bin/kiro-client-waf.prev" {
		t.Errorf("BackupPath = %q, want %q", state.BackupPath, "/usr/local/bin/kiro-client-waf.prev")
	}
}

func TestOTAUpdater_HeartbeatPushTrigger(t *testing.T) {
	// Simulate a heartbeat response that triggers an immediate update check
	checkCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkCount++
		resp := updateCheckResponse{
			UpdateAvailable: false,
			Release:         nil,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	u := NewOTAUpdater(UpdaterConfig{
		MasterURL:      server.URL,
		LicenseKey:     "test-key",
		Component:      "kiro-client-waf",
		Channel:        "stable",
		CurrentVersion: "1.0.0",
		PollInterval:   10 * time.Hour, // Very long interval so only push triggers fire
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start the updater in background
	go u.Run(ctx)

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	// Trigger an immediate check via heartbeat push
	u.TriggerUpdateCheck()

	// Wait for the check to complete
	time.Sleep(200 * time.Millisecond)

	if checkCount == 0 {
		t.Error("expected at least one update check after push trigger")
	}
}
