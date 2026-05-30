// Package ban triển khai ban engine cho Client_WAF.
// Quản lý in-memory BanStore và đồng bộ với XDP blocklist file.
package ban

import "time"

// BanEngine định nghĩa interface cho ban engine operations.
// Implementations phải quản lý in-memory ban store và đồng bộ với XDP blocklist.
type BanEngine interface {
	// IsBanned kiểm tra xem IP có đang bị ban hay không.
	// Trả về true nếu IP nằm trong ban store và chưa hết hạn.
	IsBanned(ip string) bool

	// Ban thêm IP vào ban store với duration và reason cho trước.
	// IP sẽ bị chặn ở cả L7 và được đồng bộ đến XDP blocklist.
	Ban(ip string, duration time.Duration, reason string)

	// Unban xóa IP khỏi ban store.
	// IP sẽ được gỡ khỏi L7 ban nhưng có thể vẫn còn trong XDP blocklist
	// cho đến lần sync tiếp theo.
	Unban(ip string)

	// SyncToXDP đồng bộ danh sách ban hiện tại đến XDP blocklist file
	// và trigger reload XDP maps. Phải hoàn thành trong vòng 1 giây.
	SyncToXDP() error
}
