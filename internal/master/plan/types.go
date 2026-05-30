// Package plan triển khai quản lý Package_Plan cho hệ thống license.
// Hỗ trợ Community (vô thời hạn), Pro, và Enterprise với auto-downgrade khi hết hạn.
package plan

import "time"

// Plan name constants.
const (
	PlanCommunity  = "community"
	PlanPro        = "pro"
	PlanEnterprise = "enterprise"
)

// License status constants.
const (
	StatusActive     = "active"
	StatusSuspended  = "suspended"
	StatusDowngraded = "downgraded"
)

// PlanLimits định nghĩa giới hạn tài nguyên cho mỗi gói.
type PlanLimits struct {
	MaxDomains int  // Số domain bảo vệ tối đa (0 = unlimited)
	XDPEnabled bool // XDP kernel protection khả dụng
	OTAEnabled bool // Auto-update khả dụng
	RPMPerIP   int  // Rate limit per IP per minute (0 = unlimited)
}

// CommunityLimits: 1 domain, XDP tắt, OTA tắt, 60 rpm.
var CommunityLimits = PlanLimits{
	MaxDomains: 1,
	XDPEnabled: false,
	OTAEnabled: false,
	RPMPerIP:   60,
}

// ProLimits: 5 domains, XDP bật, OTA bật, 120 rpm.
var ProLimits = PlanLimits{
	MaxDomains: 5,
	XDPEnabled: true,
	OTAEnabled: true,
	RPMPerIP:   120,
}

// EnterpriseLimits: unlimited domains, XDP bật, OTA bật, unlimited rpm.
var EnterpriseLimits = PlanLimits{
	MaxDomains: 0, // 0 = unlimited
	XDPEnabled: true,
	OTAEnabled: true,
	RPMPerIP:   0, // 0 = unlimited
}

// LimitsForPlan trả về PlanLimits tương ứng với tên plan.
// Trả về CommunityLimits nếu plan không hợp lệ.
func LimitsForPlan(plan string) PlanLimits {
	switch plan {
	case PlanPro:
		return ProLimits
	case PlanEnterprise:
		return EnterpriseLimits
	default:
		return CommunityLimits
	}
}

// DowngradeEvent ghi lại sự kiện hạ cấp license về Community.
type DowngradeEvent struct {
	LicenseID    string    `json:"license_id"`
	PreviousPlan string    `json:"previous_plan"`
	DowngradedAt time.Time `json:"downgraded_at"`
	Reason       string    `json:"reason"` // "expired", "admin_downgrade"
}

// RequestedConfig mô tả cấu hình mà client yêu cầu áp dụng.
// Dùng để kiểm tra xem config có vượt giới hạn plan không.
type RequestedConfig struct {
	Domains    int  `json:"domains"`
	XDPEnabled bool `json:"xdp_enabled"`
	OTAEnabled bool `json:"ota_enabled"`
	CustomRPM  int  `json:"custom_rpm"`
}

// PlanChange ghi lại lịch sử thay đổi plan của một license.
type PlanChange struct {
	FromPlan  string    `json:"from_plan"`
	ToPlan    string    `json:"to_plan"`
	ChangedAt time.Time `json:"changed_at"`
	Reason    string    `json:"reason"` // "expired", "admin_upgrade", "admin_downgrade"
}
