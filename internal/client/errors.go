// Package client triển khai xử lý lỗi và trang lỗi có thương hiệu cho Client_WAF.
// Bao gồm: trang 502 Bad Gateway có thương hiệu, LockdownManager cho heartbeat failure,
// và graceful degradation khi blocklist file không ghi được.
package client

import (
	"log"
	"net/http"
	"sync"
	"time"
)

// Branded502HTML là trang HTML 502 Bad Gateway có thương hiệu Kiro.
// Thiết kế tông tối nhất quán với challenge pages, văn bản tiếng Việt,
// không phụ thuộc bên ngoài.
const Branded502HTML = `<!DOCTYPE html>
<html lang="vi">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Kiro - Dịch Vụ Tạm Thời Không Khả Dụng</title>
<style>
:root {
  color-scheme: dark;
  --bg: #07100f;
  --panel: #0f1a17;
  --line: #29443e;
  --text: #eef8f5;
  --muted: #a9bdb7;
  --green: #67d891;
  --cyan: #64d8c9;
  --red: #ff8a8a;
  --gradient: linear-gradient(135deg, var(--green), var(--cyan));
}

* {
  box-sizing: border-box;
  margin: 0;
  padding: 0;
}

body {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: radial-gradient(ellipse at 50% 0%, #17302b 0%, #07100f 50%, #050807 100%);
  color: var(--text);
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Inter, Roboto, Oxygen, Ubuntu, sans-serif;
  line-height: 1.5;
  padding: 16px;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

.container {
  width: 100%;
  max-width: 480px;
  min-width: 0;
}

.card {
  border: 1px solid var(--line);
  border-radius: 12px;
  background: rgba(15, 26, 23, 0.97);
  padding: 32px 28px;
  box-shadow: 0 32px 80px rgba(0, 0, 0, 0.4), 0 0 0 1px rgba(103, 216, 145, 0.04);
  backdrop-filter: blur(8px);
}

.logo {
  width: 48px;
  height: 48px;
  border-radius: 10px;
  background: var(--gradient);
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 900;
  font-size: 1.4rem;
  color: #06100e;
  margin-bottom: 20px;
  box-shadow: 0 4px 16px rgba(100, 216, 201, 0.2);
}

.error-code {
  font-size: 3rem;
  font-weight: 800;
  background: var(--gradient);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  margin-bottom: 8px;
  letter-spacing: -0.03em;
}

h1 {
  font-size: 1.5rem;
  font-weight: 700;
  margin-bottom: 12px;
  letter-spacing: -0.02em;
}

.description {
  color: var(--muted);
  font-size: 0.95rem;
  margin-bottom: 24px;
  line-height: 1.7;
}

.status-box {
  padding: 16px;
  border-radius: 8px;
  background: rgba(255, 138, 138, 0.06);
  border: 1px solid rgba(255, 138, 138, 0.15);
  margin-bottom: 20px;
}

.status-row {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 0.9rem;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--red);
  animation: pulse 1.5s infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

.status-label {
  color: var(--red);
  font-weight: 600;
}

.retry-hint {
  color: var(--muted);
  font-size: 0.85rem;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--line);
}

.footer {
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px solid var(--line);
  text-align: center;
  color: var(--muted);
  font-size: 0.8rem;
}

@media (max-width: 360px) {
  .card {
    padding: 24px 20px;
  }
  h1 {
    font-size: 1.25rem;
  }
  .error-code {
    font-size: 2.5rem;
  }
}

@media (min-width: 1920px) {
  .card {
    padding: 40px 36px;
  }
  h1 {
    font-size: 1.75rem;
  }
}
</style>
</head>
<body>
<div class="container">
<div class="card">
<div class="logo">K</div>
<div class="error-code">502</div>
<h1>Máy chủ tạm thời không khả dụng</h1>
<p class="description">Máy chủ backend hiện không thể xử lý yêu cầu của bạn. Hệ thống đang tự động thử kết nối lại. Vui lòng thử lại sau vài giây.</p>

<div class="status-box">
<div class="status-row">
<span class="status-dot"></span>
<span class="status-label">Backend không phản hồi</span>
</div>
</div>

<p class="retry-hint">Nếu sự cố tiếp tục, vui lòng liên hệ quản trị viên hệ thống.</p>

<div class="footer">Được bảo vệ bởi Kiro WAF</div>
</div>
</div>
</body>
</html>`

// ServeBranded502 ghi trang 502 Bad Gateway có thương hiệu vào ResponseWriter.
// Được sử dụng khi backend không khả dụng.
func ServeBranded502(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusBadGateway)
	_, _ = w.Write([]byte(Branded502HTML))
}

// LockdownManager quản lý trạng thái lockdown của Client_WAF.
// Khi heartbeat thất bại 3 lần liên tiếp, hệ thống vào chế độ khóa
// chặn tất cả lưu lượng ngoại trừ từ IP admin.
// Cũng quản lý trạng thái suspended (license bị tạm ngưng) và cached plan
// cho offline behavior.
type LockdownManager struct {
	mu           sync.RWMutex
	failureCount int
	locked       bool
	lockReason   string
	lockTime     time.Time
	adminIPs     []string
	suspended    bool
	cachedPlan   string
}

// NewLockdownManager tạo LockdownManager mới với danh sách IP admin được phép
// truy cập trong chế độ lockdown.
func NewLockdownManager(adminIPs []string) *LockdownManager {
	ips := make([]string, len(adminIPs))
	copy(ips, adminIPs)
	return &LockdownManager{
		adminIPs: ips,
	}
}

// Lock chuyển hệ thống vào chế độ khóa với lý do cho trước.
// Ghi log lý do khóa và timestamp cho mục đích chẩn đoán.
func (lm *LockdownManager) Lock(reason string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if !lm.locked {
		lm.locked = true
		lm.lockReason = reason
		lm.lockTime = time.Now().UTC()
		log.Printf("LOCKDOWN ACTIVATED: reason=%q time=%s", reason, lm.lockTime.Format(time.RFC3339))
	}
}

// Unlock gỡ chế độ khóa và reset failure counter.
func (lm *LockdownManager) Unlock() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lm.locked {
		log.Printf("LOCKDOWN DEACTIVATED: was_locked_since=%s reason_was=%q",
			lm.lockTime.Format(time.RFC3339), lm.lockReason)
	}
	lm.locked = false
	lm.lockReason = ""
	lm.lockTime = time.Time{}
	lm.failureCount = 0
}

// IsLocked trả về true nếu hệ thống đang ở chế độ khóa.
func (lm *LockdownManager) IsLocked() bool {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.locked
}

// IsAdminIP kiểm tra xem IP có nằm trong danh sách admin được phép hay không.
// Trả về true nếu IP được phép truy cập trong chế độ lockdown.
func (lm *LockdownManager) IsAdminIP(ip string) bool {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	for _, adminIP := range lm.adminIPs {
		if adminIP == ip {
			return true
		}
	}
	return false
}

// RecordHeartbeatFailure ghi nhận một lần heartbeat thất bại.
// Khi số lần thất bại liên tiếp >= 3, hệ thống tự động vào chế độ khóa.
func (lm *LockdownManager) RecordHeartbeatFailure() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lm.failureCount++
	log.Printf("heartbeat failure recorded: consecutive_failures=%d", lm.failureCount)

	if lm.failureCount >= 3 && !lm.locked {
		lm.locked = true
		lm.lockReason = "heartbeat_failed_3_consecutive"
		lm.lockTime = time.Now().UTC()
		log.Printf("LOCKDOWN ACTIVATED: reason=%q time=%s consecutive_failures=%d",
			lm.lockReason, lm.lockTime.Format(time.RFC3339), lm.failureCount)
	}
}

// RecordHeartbeatSuccess ghi nhận heartbeat thành công.
// Reset failure counter và unlock nếu đang locked.
func (lm *LockdownManager) RecordHeartbeatSuccess() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lm.locked {
		log.Printf("LOCKDOWN DEACTIVATED: heartbeat_success was_locked_since=%s reason_was=%q",
			lm.lockTime.Format(time.RFC3339), lm.lockReason)
		lm.locked = false
		lm.lockReason = ""
		lm.lockTime = time.Time{}
	}
	lm.failureCount = 0
}

// GetLockInfo trả về thông tin về trạng thái lockdown hiện tại.
// Hữu ích cho diagnostics và monitoring.
func (lm *LockdownManager) GetLockInfo() (locked bool, reason string, lockTime time.Time, failures int) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.locked, lm.lockReason, lm.lockTime, lm.failureCount
}

// SetSuspended thiết lập trạng thái suspended cho license.
// Khi suspended=true, Client_Node ngừng xử lý traffic và trả về trang thông báo tạm ngưng.
// Requirement 14.10: suspended là trạng thái duy nhất ngăn Client_Node hoạt động.
func (lm *LockdownManager) SetSuspended(suspended bool) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if suspended && !lm.suspended {
		log.Printf("LICENSE SUSPENDED: client will stop processing traffic and show suspension page")
	} else if !suspended && lm.suspended {
		log.Printf("LICENSE UNSUSPENDED: client resuming normal operation")
	}
	lm.suspended = suspended
}

// IsSuspended trả về true nếu license đang bị tạm ngưng.
// Khi suspended, Client_Node phải ngừng xử lý traffic.
func (lm *LockdownManager) IsSuspended() bool {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.suspended
}

// SetCachedPlan lưu plan hiện tại để sử dụng khi offline.
// Requirement 14.6: Community cached → tiếp tục hoạt động không tự vô hiệu hóa.
func (lm *LockdownManager) SetCachedPlan(plan string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.cachedPlan = plan
}

// GetCachedPlan trả về plan đã cache.
// Dùng để quyết định offline behavior: nếu Community → tiếp tục hoạt động.
func (lm *LockdownManager) GetCachedPlan() string {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.cachedPlan
}

// LogBlocklistWriteError ghi log lỗi khi không thể ghi vào blocklist file.
// Hệ thống tiếp tục hoạt động với L7 enforcement (graceful degradation).
func LogBlocklistWriteError(path string, err error) {
	log.Printf("WARNING: blocklist file write failed (graceful degradation, L7 enforcement continues): path=%s error=%v", path, err)
}
