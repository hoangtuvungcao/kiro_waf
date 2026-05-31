// Package ban triển khai ban engine cho Client_WAF.
// InMemoryBanEngine quản lý in-memory BanStore và đồng bộ với XDP blocklist file.
package ban

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// BanEntry đại diện cho một IP bị ban trong hệ thống.
type BanEntry struct {
	IP        string    `json:"ip"`
	ExpiresAt time.Time `json:"expires_at"`
	Reason    string    `json:"reason"`
}

// InMemoryBanEngine triển khai BanEngine interface với in-memory store.
// Thread-safe thông qua sync.RWMutex.
type InMemoryBanEngine struct {
	mu            sync.RWMutex
	store         map[string]BanEntry
	blocklistPath string
	syncCommand   string
	nowFunc       func() time.Time // cho phép inject thời gian trong tests
}

// NewInMemoryBanEngine tạo một InMemoryBanEngine mới.
// blocklistPath: đường dẫn đến file XDP blocklist (ví dụ: /var/lib/kiro/xdp-blocklist.txt)
// syncCommand: lệnh để đồng bộ blocklist đến XDP maps (ví dụ: xdp-sync-maps-lab)
func NewInMemoryBanEngine(blocklistPath, syncCommand string) *InMemoryBanEngine {
	return &InMemoryBanEngine{
		store:         make(map[string]BanEntry),
		blocklistPath: blocklistPath,
		syncCommand:   syncCommand,
		nowFunc:       time.Now,
	}
}

// IsBanned kiểm tra xem IP có đang bị ban hay không.
// Trả về true nếu IP nằm trong ban store và chưa hết hạn.
// Tự động xóa entry đã hết hạn khi phát hiện.
func (e *InMemoryBanEngine) IsBanned(ip string) bool {
	e.mu.RLock()
	entry, exists := e.store[ip]
	e.mu.RUnlock()

	if !exists {
		return false
	}

	now := e.nowFunc()
	if now.After(entry.ExpiresAt) {
		// Entry đã hết hạn, xóa khỏi store
		e.mu.Lock()
		// Double-check sau khi lấy write lock
		if entry, exists := e.store[ip]; exists && now.After(entry.ExpiresAt) {
			delete(e.store, ip)
		}
		e.mu.Unlock()
		return false
	}

	return true
}

// Ban thêm IP vào ban store với duration và reason cho trước.
// Đồng thời append IP vào blocklist file và trigger XDP sync.
func (e *InMemoryBanEngine) Ban(ip string, duration time.Duration, reason string) {
	now := e.nowFunc()
	entry := BanEntry{
		IP:        ip,
		ExpiresAt: now.Add(duration),
		Reason:    reason,
	}

	// Thêm vào in-memory store
	e.mu.Lock()
	e.store[ip] = entry
	e.mu.Unlock()

	// Append vào blocklist file
	e.appendToBlocklist(ip)

	// Trigger XDP sync
	if err := e.SyncToXDP(); err != nil {
		fmt.Fprintf(os.Stderr, "ban: SyncToXDP failed for ip=%s reason=%s: %v\n", ip, reason, err)
	}
}

// Unban xóa IP khỏi ban store và rebuild blocklist file.
// Đồng thời trigger XDP sync để cập nhật kernel maps.
func (e *InMemoryBanEngine) Unban(ip string) {
	e.mu.Lock()
	delete(e.store, ip)

	// Rebuild blocklist file while still holding the write lock
	e.rebuildBlocklist()
	e.mu.Unlock()

	// Sync to XDP after releasing the lock
	if err := e.SyncToXDP(); err != nil {
		fmt.Fprintf(os.Stderr, "ban: SyncToXDP failed after Unban ip=%s: %v\n", ip, err)
	}
}

// SyncToXDP thực thi sync command để đồng bộ blocklist file đến XDP kernel maps.
// Phải hoàn thành trong vòng 1 giây.
func (e *InMemoryBanEngine) SyncToXDP() error {
	if e.syncCommand == "" {
		return nil
	}

	parts := strings.Fields(e.syncCommand)
	if len(parts) == 0 {
		return nil
	}

	var cmd *exec.Cmd
	if len(parts) == 1 {
		cmd = exec.Command(parts[0])
	} else {
		cmd = exec.Command(parts[0], parts[1:]...)
	}

	// Timeout 1 giây để đảm bảo đồng bộ nhanh
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(1 * time.Second):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return fmt.Errorf("xdp sync command timed out after 1 second")
	}
}

// CleanupExpired xóa tất cả các ban entries đã hết hạn khỏi store.
// Nên được gọi định kỳ để giải phóng bộ nhớ.
// Sau khi xóa, rebuild blocklist file và sync đến XDP.
func (e *InMemoryBanEngine) CleanupExpired() {
	e.mu.Lock()

	now := e.nowFunc()
	for ip, entry := range e.store {
		if now.After(entry.ExpiresAt) {
			delete(e.store, ip)
		}
	}

	// Rebuild blocklist file while still holding the write lock
	e.rebuildBlocklist()
	e.mu.Unlock()

	// Sync to XDP after releasing the lock
	if err := e.SyncToXDP(); err != nil {
		fmt.Fprintf(os.Stderr, "ban: SyncToXDP failed after CleanupExpired: %v\n", err)
	}
}

// GetBanEntry trả về BanEntry cho IP nếu tồn tại và chưa hết hạn.
// Trả về entry và true nếu tìm thấy, zero value và false nếu không.
func (e *InMemoryBanEngine) GetBanEntry(ip string) (BanEntry, bool) {
	e.mu.RLock()
	entry, exists := e.store[ip]
	e.mu.RUnlock()

	if !exists {
		return BanEntry{}, false
	}

	now := e.nowFunc()
	if now.After(entry.ExpiresAt) {
		return BanEntry{}, false
	}

	return entry, true
}

// BannedCount trả về số lượng IP đang bị ban (chưa hết hạn).
func (e *InMemoryBanEngine) BannedCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	now := e.nowFunc()
	count := 0
	for _, entry := range e.store {
		if !now.After(entry.ExpiresAt) {
			count++
		}
	}
	return count
}

// rebuildBlocklist truncates and rewrites the blocklist file with all currently-banned
// (non-expired) IPs in IP/32 format. Must be called while holding the write lock
// (caller ensures lock is held). Graceful degradation: logs errors but doesn't crash.
// If blocklistPath is empty, returns immediately (no-op).
func (e *InMemoryBanEngine) rebuildBlocklist() {
	if e.blocklistPath == "" {
		return
	}

	now := e.nowFunc()

	// Collect all non-expired IPs
	var lines []string
	for _, entry := range e.store {
		if !now.After(entry.ExpiresAt) {
			lines = append(lines, entry.IP+"/32\n")
		}
	}

	// Truncate and rewrite the file
	f, err := os.OpenFile(e.blocklistPath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ban: cannot open blocklist file for rebuild %s: %v\n", e.blocklistPath, err)
		return
	}
	defer f.Close()

	for _, line := range lines {
		if _, err := f.WriteString(line); err != nil {
			fmt.Fprintf(os.Stderr, "ban: cannot write to blocklist file during rebuild %s: %v\n", e.blocklistPath, err)
			return
		}
	}
}

// appendToBlocklist ghi IP/32 vào cuối blocklist file.
// Nếu file không tồn tại, tạo mới.
// Nếu ghi thất bại, log lỗi nhưng không crash (graceful degradation).
func (e *InMemoryBanEngine) appendToBlocklist(ip string) {
	if e.blocklistPath == "" {
		return
	}

	f, err := os.OpenFile(e.blocklistPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		// Graceful degradation: log lỗi nhưng tiếp tục với L7 enforcement
		fmt.Fprintf(os.Stderr, "ban: cannot open blocklist file %s: %v\n", e.blocklistPath, err)
		return
	}
	defer f.Close()

	// Ghi IP/32 format cho LPM trie lookup
	line := ip + "/32\n"
	if _, err := f.WriteString(line); err != nil {
		fmt.Fprintf(os.Stderr, "ban: cannot write to blocklist file %s: %v\n", e.blocklistPath, err)
	}
}
