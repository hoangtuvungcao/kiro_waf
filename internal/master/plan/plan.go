package plan

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"kiro_waf/internal/master/models"
)

// LicenseStore định nghĩa interface truy cập database cho license.
// Cho phép PlanManager hoạt động độc lập với implementation database cụ thể.
type LicenseStore interface {
	CreateLicense(l *models.License) error
	GetLicenseByLicenseID(licenseID string) (*models.License, error)
	UpdateLicense(l *models.License) error
	ListLicenses() ([]models.License, error)
}

// HistoryStore định nghĩa interface lưu trữ lịch sử thay đổi plan.
// Tách riêng để cho phép implementation linh hoạt (in-memory, database, etc.)
type HistoryStore interface {
	// RecordPlanChange ghi lại một sự kiện thay đổi plan cho license.
	RecordPlanChange(licenseID string, change PlanChange) error

	// GetPlanHistory trả về lịch sử thay đổi plan của một license.
	GetPlanHistory(licenseID string) ([]PlanChange, error)
}

// PlanManager interface quản lý vòng đời Package_Plan cho license.
type PlanManager interface {
	// CreateCommunityLicense tạo license Community vô thời hạn cho đăng ký mới.
	CreateCommunityLicense(customerID, customerName, fingerprint string) (*models.License, error)

	// CheckExpiry kiểm tra và xử lý license hết hạn.
	// Chuyển Pro/Enterprise → Community khi ExpiresAt đã qua.
	CheckExpiry(ctx context.Context) ([]DowngradeEvent, error)

	// UpgradePlan nâng cấp license lên gói cao hơn.
	// Giữ nguyên license_key, fingerprint, history.
	UpgradePlan(licenseID string, newPlan string, validDays int) error

	// DowngradeToCommunity hạ cấp license về Community.
	// Giữ nguyên identity, xóa tính năng premium.
	DowngradeToCommunity(licenseID string) error

	// EnforcePlanLimits kiểm tra config có vượt giới hạn plan không.
	// Trả về error nếu config vượt giới hạn.
	EnforcePlanLimits(plan string, config RequestedConfig) error
}

// Manager triển khai PlanManager interface.
type Manager struct {
	store   LicenseStore
	history HistoryStore
}

// NewManager tạo Manager mới với LicenseStore đã cho.
// Nếu historyStore là nil, lịch sử thay đổi plan sẽ không được ghi.
func NewManager(store LicenseStore, opts ...ManagerOption) *Manager {
	m := &Manager{store: store}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// ManagerOption cấu hình tùy chọn cho Manager.
type ManagerOption func(*Manager)

// WithHistoryStore thiết lập HistoryStore cho Manager.
func WithHistoryStore(h HistoryStore) ManagerOption {
	return func(m *Manager) {
		m.history = h
	}
}

// CreateCommunityLicense tạo license Community vô thời hạn.
// ExpiresAt = zero value (vô thời hạn theo Go idiom).
func (m *Manager) CreateCommunityLicense(customerID, customerName, fingerprint string) (*models.License, error) {
	licenseID, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("plan: generate license_id: %w", err)
	}

	licenseKey, err := generateKey()
	if err != nil {
		return nil, fmt.Errorf("plan: generate license_key: %w", err)
	}

	now := time.Now().UTC()
	l := &models.License{
		LicenseID:       licenseID,
		LicenseKey:      licenseKey,
		CustomerID:      customerID,
		CustomerName:    customerName,
		FingerprintHash: fingerprint,
		Plan:            PlanCommunity,
		Status:          StatusActive,
		ValidDays:       0, // Vô thời hạn
		CreatedAt:       now,
		ExpiresAt:       time.Time{}, // Zero value = vô thời hạn
	}

	if err := m.store.CreateLicense(l); err != nil {
		return nil, fmt.Errorf("plan: create community license: %w", err)
	}

	return l, nil
}

// CheckExpiry quét tất cả license Pro/Enterprise có ExpiresAt < now.
// License hết hạn được chuyển về Community, trừ khi đang suspended.
// Quy tắc ưu tiên: suspended > expired.
func (m *Manager) CheckExpiry(ctx context.Context) ([]DowngradeEvent, error) {
	licenses, err := m.store.ListLicenses()
	if err != nil {
		return nil, fmt.Errorf("plan: check expiry list: %w", err)
	}

	now := time.Now().UTC()
	var events []DowngradeEvent

	for i := range licenses {
		l := &licenses[i]

		// Chỉ kiểm tra context cancellation mỗi vòng lặp
		select {
		case <-ctx.Done():
			return events, ctx.Err()
		default:
		}

		// Bỏ qua Community (không bao giờ hết hạn)
		if l.Plan == PlanCommunity {
			continue
		}

		// Bỏ qua license chưa hết hạn hoặc ExpiresAt = zero
		if l.ExpiresAt.IsZero() || l.ExpiresAt.After(now) {
			continue
		}

		// Quy tắc ưu tiên: suspended > expired
		// Nếu license bị suspended VÀ hết hạn → giữ suspended
		if l.Status == StatusSuspended {
			continue
		}

		// Thực hiện downgrade
		previousPlan := l.Plan
		l.Plan = PlanCommunity
		l.Status = StatusDowngraded
		l.ExpiresAt = time.Time{} // Vô thời hạn
		l.ValidDays = 0

		if err := m.store.UpdateLicense(l); err != nil {
			return events, fmt.Errorf("plan: downgrade license %s: %w", l.LicenseID, err)
		}

		// Ghi PlanChange vào lịch sử
		if m.history != nil {
			change := PlanChange{
				FromPlan:  previousPlan,
				ToPlan:    PlanCommunity,
				ChangedAt: now,
				Reason:    "expired",
			}
			if err := m.history.RecordPlanChange(l.LicenseID, change); err != nil {
				log.Printf("plan: record history for %s: %v", l.LicenseID, err)
			}
		}

		events = append(events, DowngradeEvent{
			LicenseID:    l.LicenseID,
			PreviousPlan: previousPlan,
			DowngradedAt: now,
			Reason:       "expired",
		})
	}

	return events, nil
}

// UpgradePlan nâng cấp license lên gói cao hơn.
// Giữ nguyên license_key và fingerprint, cập nhật plan + ExpiresAt mới.
func (m *Manager) UpgradePlan(licenseID string, newPlan string, validDays int) error {
	if newPlan != PlanPro && newPlan != PlanEnterprise {
		return fmt.Errorf("plan: invalid upgrade target: %s", newPlan)
	}

	if validDays <= 0 {
		return fmt.Errorf("plan: validDays must be positive, got %d", validDays)
	}

	l, err := m.store.GetLicenseByLicenseID(licenseID)
	if err != nil {
		return fmt.Errorf("plan: upgrade get license: %w", err)
	}
	if l == nil {
		return fmt.Errorf("plan: license not found: %s", licenseID)
	}

	now := time.Now().UTC()
	previousPlan := l.Plan
	l.Plan = newPlan
	l.Status = StatusActive
	l.ValidDays = validDays
	l.ExpiresAt = now.AddDate(0, 0, validDays)

	if err := m.store.UpdateLicense(l); err != nil {
		return fmt.Errorf("plan: upgrade license: %w", err)
	}

	// Ghi PlanChange vào lịch sử
	if m.history != nil {
		change := PlanChange{
			FromPlan:  previousPlan,
			ToPlan:    newPlan,
			ChangedAt: now,
			Reason:    "admin_upgrade",
		}
		if err := m.history.RecordPlanChange(l.LicenseID, change); err != nil {
			log.Printf("plan: record history for %s: %v", l.LicenseID, err)
		}
	}

	return nil
}

// DowngradeToCommunity hạ cấp license về Community.
// Giữ nguyên license_key, fingerprint_hash, lịch sử.
// Đặt ExpiresAt = zero (vô thời hạn), vô hiệu hóa tính năng premium.
func (m *Manager) DowngradeToCommunity(licenseID string) error {
	l, err := m.store.GetLicenseByLicenseID(licenseID)
	if err != nil {
		return fmt.Errorf("plan: downgrade get license: %w", err)
	}
	if l == nil {
		return fmt.Errorf("plan: license not found: %s", licenseID)
	}

	previousPlan := l.Plan
	l.Plan = PlanCommunity
	l.Status = StatusDowngraded
	l.ExpiresAt = time.Time{} // Vô thời hạn
	l.ValidDays = 0

	if err := m.store.UpdateLicense(l); err != nil {
		return fmt.Errorf("plan: downgrade license: %w", err)
	}

	// Ghi PlanChange vào lịch sử
	if m.history != nil {
		change := PlanChange{
			FromPlan:  previousPlan,
			ToPlan:    PlanCommunity,
			ChangedAt: time.Now().UTC(),
			Reason:    "admin_downgrade",
		}
		if err := m.history.RecordPlanChange(licenseID, change); err != nil {
			log.Printf("plan: record history for %s: %v", licenseID, err)
		}
	}

	return nil
}

// EnforcePlanLimits kiểm tra config có vượt giới hạn plan không.
// Trả về error mô tả vi phạm nếu config vượt giới hạn.
func (m *Manager) EnforcePlanLimits(planName string, config RequestedConfig) error {
	limits := LimitsForPlan(planName)

	// Kiểm tra số domain (0 = unlimited)
	if limits.MaxDomains > 0 && config.Domains > limits.MaxDomains {
		return fmt.Errorf("plan: domains %d exceeds %s limit of %d",
			config.Domains, planName, limits.MaxDomains)
	}

	// Kiểm tra XDP
	if config.XDPEnabled && !limits.XDPEnabled {
		return fmt.Errorf("plan: XDP not available in %s plan", planName)
	}

	// Kiểm tra OTA
	if config.OTAEnabled && !limits.OTAEnabled {
		return fmt.Errorf("plan: OTA not available in %s plan", planName)
	}

	// Kiểm tra RPM (0 = unlimited)
	if limits.RPMPerIP > 0 && config.CustomRPM > limits.RPMPerIP {
		return fmt.Errorf("plan: custom RPM %d exceeds %s limit of %d",
			config.CustomRPM, planName, limits.RPMPerIP)
	}

	return nil
}

// generateID tạo license_id ngẫu nhiên 16 bytes hex.
func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// generateKey tạo license_key ngẫu nhiên 32 bytes hex.
func generateKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// StartPeriodicExpiryChecker khởi động goroutine kiểm tra license hết hạn mỗi 60 giây.
// Goroutine sẽ dừng khi ctx bị cancel.
// Trả về channel nhận DowngradeEvent mỗi khi có license bị downgrade.
func (m *Manager) StartPeriodicExpiryChecker(ctx context.Context) <-chan []DowngradeEvent {
	ch := make(chan []DowngradeEvent, 1)

	go func() {
		defer close(ch)

		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				events, err := m.CheckExpiry(ctx)
				if err != nil {
					log.Printf("plan: periodic expiry check: %v", err)
					continue
				}
				if len(events) > 0 {
					select {
					case ch <- events:
					default:
						// Channel full, log and skip
						log.Printf("plan: expiry events channel full, %d events dropped", len(events))
					}
				}
			}
		}
	}()

	return ch
}
