// Package models định nghĩa các data models cho Master_Server.
// Bao gồm: License, Release, Heartbeat, AdminSession, AdminLoginAttempt.
package models

import "time"

// License đại diện cho một license key trong hệ thống.
type License struct {
	ID              int64     `json:"id"`
	LicenseID       string    `json:"license_id"`
	LicenseKey      string    `json:"license_key"`
	CustomerID      string    `json:"customer_id"`
	CustomerName    string    `json:"customer_name"`
	ClientIP        string    `json:"client_ip"`
	FingerprintHash string    `json:"fingerprint_hash"`
	Plan            string    `json:"plan"`
	Status          string    `json:"status"` // active, suspended, revoked, expired
	ValidDays       int       `json:"valid_days"`
	CreatedAt       time.Time `json:"created_at"`
	ExpiresAt       time.Time `json:"expires_at"`
	LastHeartbeat   time.Time `json:"last_heartbeat_at"`
	Notes           string    `json:"notes"`
}

// Release đại diện cho một bản phát hành artifact.
type Release struct {
	ID          int64     `json:"id"`
	Component   string    `json:"component"`
	Channel     string    `json:"channel"`
	Version     string    `json:"version"`
	ArtifactURL string    `json:"artifact_url"`
	SHA256      string    `json:"sha256"`
	Notes       string    `json:"notes"`
	MinVersion  string    `json:"min_version"`
	CreatedAt   time.Time `json:"created_at"`
}

// Heartbeat đại diện cho một bản ghi heartbeat từ client node.
type Heartbeat struct {
	ID              int64          `json:"id"`
	LicenseID       string         `json:"license_id"`
	NodeID          string         `json:"node_id"`
	ClientIP        string         `json:"client_ip"`
	FingerprintHash string         `json:"fingerprint_hash"`
	Stats           map[string]any `json:"stats"`
	CreatedAt       time.Time      `json:"created_at"`
}

// AdminSession đại diện cho một phiên đăng nhập admin.
type AdminSession struct {
	ID           int64     `json:"id"`
	SessionToken string    `json:"session_token"`
	IP           string    `json:"ip"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// AdminLoginAttempt ghi lại một lần thử đăng nhập admin (cho brute-force protection).
type AdminLoginAttempt struct {
	ID          int64     `json:"id"`
	IP          string    `json:"ip"`
	Success     bool      `json:"success"`
	AttemptedAt time.Time `json:"attempted_at"`
}
