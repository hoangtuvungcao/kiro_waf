package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"kiro_waf/master-server/models"
)

// CreateLicense inserts a new license into the database.
func (d *DB) CreateLicense(l *models.License) error {
	query := `
		INSERT INTO licenses (license_id, license_key, customer_id, customer_name, client_ip,
			fingerprint_hash, plan, status, valid_days, created_at, expires_at, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := d.conn.Exec(query,
		l.LicenseID, l.LicenseKey, l.CustomerID, l.CustomerName, l.ClientIP,
		l.FingerprintHash, l.Plan, l.Status, l.ValidDays,
		l.CreatedAt, l.ExpiresAt, l.Notes,
	)
	if err != nil {
		return fmt.Errorf("db: create license: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("db: create license last id: %w", err)
	}
	l.ID = id
	return nil
}

// GetLicenseByID retrieves a license by its database ID.
func (d *DB) GetLicenseByID(id int64) (*models.License, error) {
	query := `SELECT id, license_id, license_key, customer_id, customer_name, client_ip,
		fingerprint_hash, plan, status, valid_days, created_at, expires_at,
		COALESCE(last_heartbeat_at, ''), notes
		FROM licenses WHERE id = ?`

	l := &models.License{}
	var lastHB string
	err := d.conn.QueryRow(query, id).Scan(
		&l.ID, &l.LicenseID, &l.LicenseKey, &l.CustomerID, &l.CustomerName,
		&l.ClientIP, &l.FingerprintHash, &l.Plan, &l.Status, &l.ValidDays,
		&l.CreatedAt, &l.ExpiresAt, &lastHB, &l.Notes,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("db: get license by id: %w", err)
	}
	if lastHB != "" {
		l.LastHeartbeat, _ = time.Parse(time.DateTime, lastHB)
	}
	return l, nil
}

// GetLicenseByKey retrieves a license by its license_key.
func (d *DB) GetLicenseByKey(key string) (*models.License, error) {
	query := `SELECT id, license_id, license_key, customer_id, customer_name, client_ip,
		fingerprint_hash, plan, status, valid_days, created_at, expires_at,
		COALESCE(last_heartbeat_at, ''), notes
		FROM licenses WHERE license_key = ?`

	l := &models.License{}
	var lastHB string
	err := d.conn.QueryRow(query, key).Scan(
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

// GetLicenseByLicenseID retrieves a license by its license_id field.
func (d *DB) GetLicenseByLicenseID(licenseID string) (*models.License, error) {
	query := `SELECT id, license_id, license_key, customer_id, customer_name, client_ip,
		fingerprint_hash, plan, status, valid_days, created_at, expires_at,
		COALESCE(last_heartbeat_at, ''), notes
		FROM licenses WHERE license_id = ?`

	l := &models.License{}
	var lastHB string
	err := d.conn.QueryRow(query, licenseID).Scan(
		&l.ID, &l.LicenseID, &l.LicenseKey, &l.CustomerID, &l.CustomerName,
		&l.ClientIP, &l.FingerprintHash, &l.Plan, &l.Status, &l.ValidDays,
		&l.CreatedAt, &l.ExpiresAt, &lastHB, &l.Notes,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("db: get license by license_id: %w", err)
	}
	if lastHB != "" {
		l.LastHeartbeat, _ = time.Parse(time.DateTime, lastHB)
	}
	return l, nil
}

// ListLicenses returns all licenses ordered by creation date descending.
func (d *DB) ListLicenses() ([]models.License, error) {
	query := `SELECT id, license_id, license_key, customer_id, customer_name, client_ip,
		fingerprint_hash, plan, status, valid_days, created_at, expires_at,
		COALESCE(last_heartbeat_at, ''), notes
		FROM licenses ORDER BY created_at DESC`

	rows, err := d.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("db: list licenses: %w", err)
	}
	defer rows.Close()

	var licenses []models.License
	for rows.Next() {
		var l models.License
		var lastHB string
		if err := rows.Scan(
			&l.ID, &l.LicenseID, &l.LicenseKey, &l.CustomerID, &l.CustomerName,
			&l.ClientIP, &l.FingerprintHash, &l.Plan, &l.Status, &l.ValidDays,
			&l.CreatedAt, &l.ExpiresAt, &lastHB, &l.Notes,
		); err != nil {
			return nil, fmt.Errorf("db: list licenses scan: %w", err)
		}
		if lastHB != "" {
			l.LastHeartbeat, _ = time.Parse(time.DateTime, lastHB)
		}
		licenses = append(licenses, l)
	}
	return licenses, rows.Err()
}

// UpdateLicense updates mutable fields of a license.
func (d *DB) UpdateLicense(l *models.License) error {
	query := `UPDATE licenses SET
		customer_name = ?, client_ip = ?, fingerprint_hash = ?,
		plan = ?, status = ?, valid_days = ?, expires_at = ?, notes = ?
		WHERE id = ?`

	result, err := d.conn.Exec(query,
		l.CustomerName, l.ClientIP, l.FingerprintHash,
		l.Plan, l.Status, l.ValidDays, l.ExpiresAt, l.Notes,
		l.ID,
	)
	if err != nil {
		return fmt.Errorf("db: update license: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("db: update license: not found")
	}
	return nil
}

// DeleteLicense removes a license by its database ID.
func (d *DB) DeleteLicense(id int64) error {
	result, err := d.conn.Exec("DELETE FROM licenses WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("db: delete license: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("db: delete license: not found")
	}
	return nil
}

// RenewLicense extends the expiration date by the license's valid_days from now.
func (d *DB) RenewLicense(id int64) error {
	query := `UPDATE licenses SET
		expires_at = datetime('now', '+' || valid_days || ' days'),
		status = 'active'
		WHERE id = ?`

	result, err := d.conn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("db: renew license: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("db: renew license: not found")
	}
	return nil
}

// RotateLicenseKey generates a new license_key for the given license.
func (d *DB) RotateLicenseKey(id int64) (string, error) {
	newKey, err := generateKey()
	if err != nil {
		return "", fmt.Errorf("db: rotate key generate: %w", err)
	}

	query := `UPDATE licenses SET license_key = ? WHERE id = ?`
	result, err := d.conn.Exec(query, newKey, id)
	if err != nil {
		return "", fmt.Errorf("db: rotate license key: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return "", fmt.Errorf("db: rotate license key: not found")
	}
	return newKey, nil
}

// RevokeLicense sets the license status to "revoked".
func (d *DB) RevokeLicense(id int64) error {
	query := `UPDATE licenses SET status = 'revoked' WHERE id = ?`
	result, err := d.conn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("db: revoke license: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("db: revoke license: not found")
	}
	return nil
}

// UpdateLicenseHeartbeat updates the last_heartbeat_at timestamp for a license.
func (d *DB) UpdateLicenseHeartbeat(licenseID string, t time.Time) error {
	query := `UPDATE licenses SET last_heartbeat_at = ? WHERE license_id = ?`
	_, err := d.conn.Exec(query, t, licenseID)
	if err != nil {
		return fmt.Errorf("db: update license heartbeat: %w", err)
	}
	return nil
}

// CountLicensesByStatus returns a map of status → count for all licenses.
func (d *DB) CountLicensesByStatus() (map[string]int, error) {
	query := `SELECT status, COUNT(*) FROM licenses GROUP BY status`
	rows, err := d.conn.Query(query)
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

// generateKey produces a random 32-byte hex-encoded key.
func generateKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
