package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// AlertSender sends alerts to the Master Server when failures escalate.
type AlertSender struct {
	// MasterURL is the base URL of the Master Server (e.g., "https://firewall.vpsgen.com").
	MasterURL string

	// LicenseKey is used for authentication with the Master Server.
	LicenseKey string

	// NodeID identifies this client node.
	NodeID string

	// client is the HTTP client used for sending alerts.
	client *http.Client
}

// AlertPayload is the JSON body sent to the Master Server alert endpoint.
type AlertPayload struct {
	Type      string    `json:"type"`
	NodeID    string    `json:"node_id"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Severity  string    `json:"severity"` // "warning", "critical"
}

// AlertSenderConfig holds configuration for the AlertSender.
type AlertSenderConfig struct {
	MasterURL  string
	LicenseKey string
	NodeID     string
	Timeout    time.Duration
}

// NewAlertSender creates a new AlertSender with the given configuration.
func NewAlertSender(cfg AlertSenderConfig) *AlertSender {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	return &AlertSender{
		MasterURL:  cfg.MasterURL,
		LicenseKey: cfg.LicenseKey,
		NodeID:     cfg.NodeID,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// SendAlert sends an alert to the Master Server.
// It is non-blocking and logs errors rather than returning them,
// since alert delivery should not block the health monitor.
func (as *AlertSender) SendAlert(alertType string, message string) {
	if as.MasterURL == "" {
		log.Printf("Health monitor: alert [%s] (no master configured): %s", alertType, message)
		return
	}

	go as.sendAlertAsync(alertType, message)
}

// sendAlertAsync performs the actual HTTP request to send the alert.
func (as *AlertSender) sendAlertAsync(alertType string, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	severity := "warning"
	if alertType == "restart_failed" || alertType == "xdp_reload_failed" {
		severity = "critical"
	}

	payload := AlertPayload{
		Type:      alertType,
		NodeID:    as.NodeID,
		Message:   message,
		Timestamp: time.Now(),
		Severity:  severity,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Health monitor: failed to marshal alert payload: %v", err)
		return
	}

	url := fmt.Sprintf("%s/api/v1/alert", as.MasterURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Printf("Health monitor: failed to create alert request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-License-Key", as.LicenseKey)

	resp, err := as.client.Do(req)
	if err != nil {
		log.Printf("Health monitor: failed to send alert to master: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("Health monitor: alert [%s] sent to master successfully", alertType)
	} else {
		log.Printf("Health monitor: master returned %d for alert [%s]", resp.StatusCode, alertType)
	}
}
