package monitor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewAlertSender(t *testing.T) {
	sender := NewAlertSender(AlertSenderConfig{
		MasterURL:  "https://firewall.vpsgen.com",
		LicenseKey: "test-key",
		NodeID:     "node-1",
		Timeout:    5 * time.Second,
	})

	if sender.MasterURL != "https://firewall.vpsgen.com" {
		t.Errorf("MasterURL = %q, want %q", sender.MasterURL, "https://firewall.vpsgen.com")
	}
	if sender.LicenseKey != "test-key" {
		t.Errorf("LicenseKey = %q, want %q", sender.LicenseKey, "test-key")
	}
	if sender.NodeID != "node-1" {
		t.Errorf("NodeID = %q, want %q", sender.NodeID, "node-1")
	}
}

func TestNewAlertSenderDefaultTimeout(t *testing.T) {
	sender := NewAlertSender(AlertSenderConfig{
		MasterURL: "https://firewall.vpsgen.com",
	})

	if sender.client.Timeout != 10*time.Second {
		t.Errorf("default timeout = %v, want 10s", sender.client.Timeout)
	}
}

func TestAlertSenderSendAlert_NoMasterURL(t *testing.T) {
	sender := NewAlertSender(AlertSenderConfig{
		MasterURL: "", // No master configured
	})

	// Should not panic, just log
	sender.SendAlert("test_alert", "test message")
}

func TestAlertSenderSendAlert_Success(t *testing.T) {
	var receivedPayload AlertPayload
	var receivedLicenseKey string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/alert" {
			t.Errorf("request path = %q, want %q", r.URL.Path, "/api/v1/alert")
		}
		if r.Method != http.MethodPost {
			t.Errorf("request method = %q, want %q", r.Method, http.MethodPost)
		}

		receivedLicenseKey = r.Header.Get("X-License-Key")

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&receivedPayload); err != nil {
			t.Errorf("failed to decode payload: %v", err)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := NewAlertSender(AlertSenderConfig{
		MasterURL:  srv.URL,
		LicenseKey: "my-license-key",
		NodeID:     "node-42",
		Timeout:    5 * time.Second,
	})

	// SendAlert is async, so we need to wait a bit
	sender.sendAlertAsync("restart_failed", "service restart failed 3 times")

	if receivedPayload.Type != "restart_failed" {
		t.Errorf("payload type = %q, want %q", receivedPayload.Type, "restart_failed")
	}
	if receivedPayload.NodeID != "node-42" {
		t.Errorf("payload node_id = %q, want %q", receivedPayload.NodeID, "node-42")
	}
	if receivedPayload.Severity != "critical" {
		t.Errorf("payload severity = %q, want %q", receivedPayload.Severity, "critical")
	}
	if receivedPayload.Message != "service restart failed 3 times" {
		t.Errorf("payload message = %q, want expected", receivedPayload.Message)
	}
	if receivedLicenseKey != "my-license-key" {
		t.Errorf("license key header = %q, want %q", receivedLicenseKey, "my-license-key")
	}
	if receivedPayload.Timestamp.IsZero() {
		t.Error("payload timestamp should not be zero")
	}
}

func TestAlertSenderSendAlert_XDPReloadFailed(t *testing.T) {
	var receivedPayload AlertPayload

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		_ = decoder.Decode(&receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := NewAlertSender(AlertSenderConfig{
		MasterURL:  srv.URL,
		LicenseKey: "key",
		NodeID:     "node-1",
	})

	sender.sendAlertAsync("xdp_reload_failed", "XDP reload failed 3 times on eth0")

	if receivedPayload.Severity != "critical" {
		t.Errorf("severity for xdp_reload_failed = %q, want %q", receivedPayload.Severity, "critical")
	}
}

func TestAlertSenderSendAlert_WarningSeverity(t *testing.T) {
	var receivedPayload AlertPayload

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		_ = decoder.Decode(&receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := NewAlertSender(AlertSenderConfig{
		MasterURL:  srv.URL,
		LicenseKey: "key",
		NodeID:     "node-1",
	})

	sender.sendAlertAsync("ddos_detected", "DDoS attack detected")

	if receivedPayload.Severity != "warning" {
		t.Errorf("severity for ddos_detected = %q, want %q", receivedPayload.Severity, "warning")
	}
}

func TestAlertSenderSendAlert_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	sender := NewAlertSender(AlertSenderConfig{
		MasterURL:  srv.URL,
		LicenseKey: "key",
		NodeID:     "node-1",
	})

	// Should not panic even when server returns error
	sender.sendAlertAsync("test", "test message")
}
