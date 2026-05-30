package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"kiro_waf/internal/master/models"
)

// GetLicenseByKeyCtx retrieves a license by its license_key using the provided context.
// If the context deadline is exceeded, returns a context.DeadlineExceeded error.
func (d *DB) GetLicenseByKeyCtx(ctx context.Context, key string) (*models.License, error) {
	query := `SELECT id, license_id, license_key, customer_id, customer_name, client_ip,
		fingerprint_hash, plan, status, valid_days, created_at, expires_at,
		COALESCE(last_heartbeat_at, ''), notes
		FROM licenses WHERE license_key = ?`

	l := &models.License{}
	var lastHB string
	err := d.conn.QueryRowContext(ctx, query, key).Scan(
		&l.ID, &l.LicenseID, &l.LicenseKey, &l.CustomerID, &l.CustomerName,
		&l.ClientIP, &l.FingerprintHash, &l.Plan, &l.Status, &l.ValidDays,
		&l.CreatedAt, &l.ExpiresAt, &lastHB, &l.Notes,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("db: get license by key: %w", err)
	}
	if lastHB != "" {
		l.LastHeartbeat, _ = time.Parse(time.DateTime, lastHB)
	}
	return l, nil
}

// UpdateLicenseHeartbeatCtx updates the last_heartbeat_at timestamp using the provided context.
func (d *DB) UpdateLicenseHeartbeatCtx(ctx context.Context, licenseID string, t time.Time) error {
	query := `UPDATE licenses SET last_heartbeat_at = ? WHERE license_id = ?`
	_, err := d.conn.ExecContext(ctx, query, t, licenseID)
	if err != nil {
		return fmt.Errorf("db: update license heartbeat: %w", err)
	}
	return nil
}

// CountLicensesByStatusCtx returns a map of status → count using the provided context.
func (d *DB) CountLicensesByStatusCtx(ctx context.Context) (map[string]int, error) {
	query := `SELECT status, COUNT(*) FROM licenses GROUP BY status`
	rows, err := d.conn.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("db: count licenses by status: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("db: count licenses by status scan: %w", err)
		}
		counts[status] = count
	}
	return counts, rows.Err()
}

// UpdateLicenseStatusCtx updates the status field of a license by its license_id using the provided context.
func (d *DB) UpdateLicenseStatusCtx(ctx context.Context, licenseID string, status string) error {
	query := `UPDATE licenses SET status = ? WHERE license_id = ?`
	_, err := d.conn.ExecContext(ctx, query, status, licenseID)
	if err != nil {
		return fmt.Errorf("db: update license status: %w", err)
	}
	return nil
}

// GetLicenseByIDCtx retrieves a license by its license_id field using the provided context.
// This is the fallback lookup when a user provides license_id instead of license_key
// (e.g., when copying from admin panel which displays license_id).
func (d *DB) GetLicenseByIDCtx(ctx context.Context, licenseID string) (*models.License, error) {
	query := `SELECT id, license_id, license_key, customer_id, customer_name, client_ip,
		fingerprint_hash, plan, status, valid_days, created_at, expires_at,
		COALESCE(last_heartbeat_at, ''), notes
		FROM licenses WHERE license_id = ?`

	l := &models.License{}
	var lastHB string
	err := d.conn.QueryRowContext(ctx, query, licenseID).Scan(
		&l.ID, &l.LicenseID, &l.LicenseKey, &l.CustomerID, &l.CustomerName,
		&l.ClientIP, &l.FingerprintHash, &l.Plan, &l.Status, &l.ValidDays,
		&l.CreatedAt, &l.ExpiresAt, &lastHB, &l.Notes,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("db: get license by id ctx: %w", err)
	}
	if lastHB != "" {
		l.LastHeartbeat, _ = time.Parse(time.DateTime, lastHB)
	}
	return l, nil
}
