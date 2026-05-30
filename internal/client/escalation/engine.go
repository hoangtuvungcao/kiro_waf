// Package escalation triển khai Escalation Engine cho Client_WAF.
// EscalationEngine quản lý per-IP escalation state in-memory, xác định challenge level (0-4)
// dựa trên admin allowlist, failed challenge count, và cooldown-based de-escalation.
package escalation

import (
	"sync"
	"time"
)

// EscalationConfig chứa cấu hình cho EscalationEngine.
type EscalationConfig struct {
	// FailureThreshold: số lần thất bại trong FailureWindow trước khi escalate.
	FailureThreshold int
	// FailureWindow: khoảng thời gian tính failure count (ví dụ: 5 phút).
	FailureWindow time.Duration
	// CooldownDuration: thời gian không có violation trước khi de-escalate 1 level.
	CooldownDuration time.Duration
}

// IPState lưu trạng thái escalation cho một IP.
type IPState struct {
	Level          int       // 0=none, 1=transparent, 2=PoW, 3=hold, 4=ban
	FailureCount   int       // số lần thất bại trong window hiện tại
	LastFailure    time.Time // thời điểm failure gần nhất
	LastEscalation time.Time // thời điểm escalation gần nhất
	LastActivityAt time.Time // thời điểm hoạt động gần nhất (dùng cho cleanup)
}

// EscalationEngine quản lý per-IP escalation state in-memory.
// Thread-safe thông qua sync.RWMutex.
type EscalationEngine struct {
	mu             sync.RWMutex
	states         map[string]*IPState
	adminAllowlist map[string]bool
	config         EscalationConfig
	nowFunc        func() time.Time // cho phép inject thời gian trong tests
}

// NewEscalationEngine tạo một EscalationEngine mới với cấu hình và danh sách admin IPs.
func NewEscalationEngine(config EscalationConfig, adminIPs []string) *EscalationEngine {
	allowlist := make(map[string]bool, len(adminIPs))
	for _, ip := range adminIPs {
		allowlist[ip] = true
	}

	return &EscalationEngine{
		states:         make(map[string]*IPState),
		adminAllowlist: allowlist,
		config:         config,
		nowFunc:        time.Now,
	}
}

// GetLevel trả về challenge level hiện tại cho IP.
// - Trả về 0 cho admin IPs (bypass tất cả challenges).
// - Kiểm tra cooldown-based de-escalation trước khi trả về level.
// - Trả về 1 (transparent challenge) nếu IP chưa có state (new visitor).
func (e *EscalationEngine) GetLevel(ip string) int {
	// Admin IPs luôn bypass
	e.mu.RLock()
	if e.adminAllowlist[ip] {
		e.mu.RUnlock()
		return 0
	}

	state, exists := e.states[ip]
	if !exists {
		e.mu.RUnlock()
		return 1 // New visitor → transparent challenge
	}

	// Copy state data under read lock
	level := state.Level
	lastEscalation := state.LastEscalation
	e.mu.RUnlock()

	// Kiểm tra cooldown-based de-escalation
	now := e.nowFunc()
	if level > 0 && !lastEscalation.IsZero() {
		elapsed := now.Sub(lastEscalation)
		// De-escalate 1 level cho mỗi cooldown period đã trôi qua
		deEscalations := int(elapsed / e.config.CooldownDuration)
		if deEscalations > 0 {
			newLevel := level - deEscalations
			if newLevel < 0 {
				newLevel = 0
			}
			// Cập nhật state với write lock
			e.mu.Lock()
			// Re-check state vẫn tồn tại (có thể bị cleanup)
			if s, ok := e.states[ip]; ok {
				s.Level = newLevel
				s.LastEscalation = now
				s.LastActivityAt = now
				level = newLevel
			}
			e.mu.Unlock()
		}
	}

	return level
}

// RecordFailure ghi nhận một challenge failure cho IP.
// Tăng failure count và escalate khi vượt threshold trong failure window.
func (e *EscalationEngine) RecordFailure(ip string, challengeType string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := e.nowFunc()

	state, exists := e.states[ip]
	if !exists {
		state = &IPState{
			Level: 1, // Mặc định level 1 cho new visitor
		}
		e.states[ip] = state
	}

	// Reset failure count nếu ngoài failure window
	if !state.LastFailure.IsZero() && now.Sub(state.LastFailure) > e.config.FailureWindow {
		state.FailureCount = 0
	}

	state.FailureCount++
	state.LastFailure = now
	state.LastActivityAt = now

	// Escalate khi vượt threshold
	if state.FailureCount > e.config.FailureThreshold {
		if state.Level < 4 {
			state.Level++
			state.LastEscalation = now
			state.FailureCount = 0 // Reset counter sau khi escalate
		}
	}
}

// RecordSuccess ghi nhận một challenge success cho IP.
// Reset failure count nhưng KHÔNG de-escalate ngay lập tức.
// De-escalation chỉ xảy ra qua cooldown mechanism trong GetLevel.
func (e *EscalationEngine) RecordSuccess(ip string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := e.nowFunc()

	state, exists := e.states[ip]
	if !exists {
		return // Không có state → không cần làm gì
	}

	state.FailureCount = 0
	state.LastActivityAt = now
}

// Cleanup xóa các entries cũ hơn 2× CooldownDuration khỏi states map.
// Nên được gọi định kỳ để giải phóng bộ nhớ.
func (e *EscalationEngine) Cleanup() {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := e.nowFunc()
	staleThreshold := 2 * e.config.CooldownDuration

	for ip, state := range e.states {
		if now.Sub(state.LastActivityAt) > staleThreshold {
			delete(e.states, ip)
		}
	}
}

// SetNowFunc sets the time function used by the engine (for testing).
func (e *EscalationEngine) SetNowFunc(fn func() time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.nowFunc = fn
}
