// Package main triển khai heartbeat loop và update check loop cho Client_WAF.
// StartHeartbeatLoop gửi trạng thái đến Master_Server theo interval, xử lý lockdown khi thất bại liên tiếp.
// StartUpdateCheckLoop kiểm tra phiên bản mới và ghi thông báo ra stdout.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

// HeartbeatConfig chứa cấu hình cho heartbeat loop.
type HeartbeatConfig struct {
	// MasterURL là URL của Master_Server (ví dụ: "https://firewall.vpsgen.com").
	MasterURL string

	// LicenseKey là license key của client node.
	LicenseKey string

	// NodeID là định danh duy nhất của node.
	NodeID string

	// FingerprintHash là hash fingerprint của máy.
	FingerprintHash string

	// Interval là khoảng thời gian giữa các lần gửi heartbeat. Mặc định 60s.
	Interval time.Duration

	// Stats là thông tin bổ sung gửi kèm heartbeat (optional).
	Stats map[string]any
}

// UpdateCheckConfig chứa cấu hình cho update check loop.
type UpdateCheckConfig struct {
	// MasterURL là URL của Master_Server.
	MasterURL string

	// Component là tên component cần kiểm tra cập nhật (ví dụ: "kiro-client-waf").
	Component string

	// Channel là kênh cập nhật (ví dụ: "stable").
	Channel string

	// CurrentVersion là phiên bản hiện tại của component.
	CurrentVersion string

	// Interval là khoảng thời gian giữa các lần kiểm tra. Mặc định 300s.
	Interval time.Duration
}

// heartbeatLoopRequest là payload gửi đến Master_Server khi heartbeat.
type heartbeatLoopRequest struct {
	LicenseKey      string         `json:"license_key"`
	NodeID          string         `json:"node_id"`
	FingerprintHash string         `json:"fingerprint_hash"`
	Stats           map[string]any `json:"stats"`
}

// heartbeatLoopResponse là response từ Master_Server cho heartbeat.
type heartbeatLoopResponse struct {
	Valid      bool              `json:"valid"`
	Lock       bool              `json:"lock"`
	Reason     string            `json:"reason"`
	Status     string            `json:"status"`
	Plan       string            `json:"plan"`
	ExpiresAt  string            `json:"expires_at"`
	PlanConfig *loopPlanConfig   `json:"plan_config"`
}

// loopPlanConfig là plan configuration nhận từ Master_Server.
type loopPlanConfig struct {
	RPMPerIP   int  `json:"rpm_per_ip"`
	SubnetRPM  int  `json:"subnet_rpm"`
	MaxDomains int  `json:"max_domains"`
	XDPEnabled bool `json:"xdp_enabled"`
	OTAEnabled bool `json:"ota_enabled"`
}

// updateLoopRequest là payload gửi đến Master_Server khi kiểm tra cập nhật.
type updateLoopRequest struct {
	Component      string `json:"component"`
	Channel        string `json:"channel"`
	CurrentVersion string `json:"current_version"`
}

// updateLoopResponse là response từ Master_Server cho update check.
type updateLoopResponse struct {
	UpdateAvailable bool         `json:"update_available"`
	Release         *ReleaseInfo `json:"release"`
}

// ReleaseInfo chứa thông tin về bản phát hành mới.
type ReleaseInfo struct {
	Component   string `json:"component"`
	Version     string `json:"version"`
	ArtifactURL string `json:"artifact_url"`
	SHA256      string `json:"sha256"`
	Notes       string `json:"notes"`
}

// StartHeartbeatLoop khởi chạy goroutine heartbeat gửi trạng thái đến Master_Server theo interval.
// Gửi POST đến master_url/api/v1/heartbeat mỗi interval.
// Khi heartbeat thành công (valid: true): gọi lockdown.RecordHeartbeatSuccess().
// Khi heartbeat thất bại (valid: false, lock: true) hoặc HTTP error: gọi lockdown.RecordHeartbeatFailure().
// Chấp nhận context để graceful shutdown.
func StartHeartbeatLoop(ctx context.Context, config HeartbeatConfig, lockdown *LockdownManager) {
	interval := config.Interval
	if interval <= 0 {
		interval = 60 * time.Second
	}

	client := &http.Client{Timeout: 10 * time.Second}

	// Gửi heartbeat ngay lập tức lần đầu
	sendHeartbeat(client, config, lockdown)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sendHeartbeat(client, config, lockdown)
		}
	}
}

// sendHeartbeat gửi một heartbeat request đến Master_Server.
// Xử lý plan enforcement:
// - plan=Community → hoạt động bình thường với tính năng cơ bản (rate-limit 60 rpm/IP)
// - status=suspended → kích hoạt lockdown với lý do "license_suspended"
// - Offline behavior: nếu không kết nối được Master và license Community cached → tiếp tục hoạt động
func sendHeartbeat(client *http.Client, config HeartbeatConfig, lockdown *LockdownManager) {
	endpoint := strings.TrimRight(config.MasterURL, "/") + "/api/v1/heartbeat"

	payload := heartbeatLoopRequest{
		LicenseKey:      config.LicenseKey,
		NodeID:          config.NodeID,
		FingerprintHash: config.FingerprintHash,
		Stats:           config.Stats,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		// Network/marshal error: if Community plan cached, continue operating (Req 14.6)
		if lockdown.GetCachedPlan() == "community" {
			log.Printf("heartbeat: marshal error but community plan cached, continuing operation")
			return
		}
		lockdown.RecordHeartbeatFailure()
		return
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		if lockdown.GetCachedPlan() == "community" {
			log.Printf("heartbeat: request build error but community plan cached, continuing operation")
			return
		}
		lockdown.RecordHeartbeatFailure()
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		// Master unreachable: if Community plan cached, continue operating (Req 14.6)
		if lockdown.GetCachedPlan() == "community" {
			log.Printf("heartbeat: master unreachable but community plan cached, continuing operation")
			return
		}
		lockdown.RecordHeartbeatFailure()
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if lockdown.GetCachedPlan() == "community" {
			log.Printf("heartbeat: server returned status %d but community plan cached, continuing operation", resp.StatusCode)
			return
		}
		lockdown.RecordHeartbeatFailure()
		return
	}

	var result heartbeatLoopResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		if lockdown.GetCachedPlan() == "community" {
			log.Printf("heartbeat: decode error but community plan cached, continuing operation")
			return
		}
		lockdown.RecordHeartbeatFailure()
		return
	}

	// Handle hard lock (revoked license).
	if result.Lock {
		log.Printf("heartbeat: server requested lock, reason=%s", result.Reason)
		lockdown.RecordHeartbeatFailure()
		return
	}

	// Handle suspended status (Req 14.10): stop processing traffic, show suspension page.
	if result.Status == "suspended" {
		log.Printf("heartbeat: license suspended, activating suspension mode")
		lockdown.SetSuspended(true)
		// Cache the plan for offline behavior
		if result.Plan != "" {
			lockdown.SetCachedPlan(result.Plan)
		}
		return
	}

	// Clear suspension if previously suspended and now active/downgraded.
	lockdown.SetSuspended(false)

	if !result.Valid {
		// Not valid but not locked — expired license, downgraded to community.
		log.Printf("heartbeat: license not valid (status=%s plan=%s), using plan_config from server", result.Status, result.Plan)
	}

	// Cache the plan for offline behavior (Req 14.6).
	if result.Plan != "" {
		lockdown.SetCachedPlan(result.Plan)
	}

	// Apply plan config if provided (Req 14.5).
	if result.PlanConfig != nil {
		log.Printf("heartbeat: plan=%s plan_config received rpm_per_ip=%d subnet_rpm=%d max_domains=%d xdp=%v ota=%v",
			result.Plan, result.PlanConfig.RPMPerIP, result.PlanConfig.SubnetRPM,
			result.PlanConfig.MaxDomains, result.PlanConfig.XDPEnabled, result.PlanConfig.OTAEnabled)
	}

	// Heartbeat thành công (or downgraded but not locked)
	lockdown.RecordHeartbeatSuccess()
}

// StartUpdateCheckLoop khởi chạy goroutine kiểm tra phiên bản mới theo interval.
// Gửi POST đến master_url/api/v1/update/check mỗi interval.
// Khi có bản cập nhật mới: ghi thông báo ra stdout với component, current version, new version, artifact URL.
// Chấp nhận context để graceful shutdown.
func StartUpdateCheckLoop(ctx context.Context, config UpdateCheckConfig) {
	interval := config.Interval
	if interval <= 0 {
		interval = 300 * time.Second
	}

	client := &http.Client{Timeout: 10 * time.Second}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkForUpdate(client, config)
		}
	}
}

// checkForUpdate gửi một update check request đến Master_Server.
func checkForUpdate(client *http.Client, config UpdateCheckConfig) {
	endpoint := strings.TrimRight(config.MasterURL, "/") + "/api/v1/update/check"

	payload := updateLoopRequest{
		Component:      config.Component,
		Channel:        config.Channel,
		CurrentVersion: config.CurrentVersion,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("update check: marshal failed: %v", err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		log.Printf("update check: request build failed: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("update check: http error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("update check: server returned status %d", resp.StatusCode)
		return
	}

	var result updateLoopResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("update check: decode failed: %v", err)
		return
	}

	if !result.UpdateAvailable || result.Release == nil {
		return
	}

	// Ghi thông báo ra stdout khi có bản cập nhật mới
	log.Printf("UPDATE AVAILABLE: component=%s current_version=%s new_version=%s artifact_url=%s",
		result.Release.Component, config.CurrentVersion,
		result.Release.Version, result.Release.ArtifactURL)
}
