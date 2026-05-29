package db

import (
	"encoding/json"
	"fmt"
	"time"

	"kiro_waf/master-server/models"
)

// LogHeartbeat inserts a heartbeat record into the database.
func (d *DB) LogHeartbeat(h *models.Heartbeat) error {
	statsJSON, err := json.Marshal(h.Stats)
	if err != nil {
		statsJSON = []byte("{}")
	}

	query := `INSERT INTO heartbeats (license_id, node_id, client_ip, fingerprint_hash, stats_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`

	result, err := d.conn.Exec(query,
		h.LicenseID, h.NodeID, h.ClientIP, h.FingerprintHash,
		string(statsJSON), h.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("db: log heartbeat: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("db: log heartbeat last id: %w", err)
	}
	h.ID = id
	return nil
}

// ListHeartbeats returns heartbeats ordered by creation date descending, limited to n records.
func (d *DB) ListHeartbeats(limit int) ([]models.Heartbeat, error) {
	query := `SELECT id, license_id, node_id, client_ip, fingerprint_hash, stats_json, created_at
		FROM heartbeats ORDER BY created_at DESC LIMIT ?`

	rows, err := d.conn.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("db: list heartbeats: %w", err)
	}
	defer rows.Close()

	var heartbeats []models.Heartbeat
	for rows.Next() {
		var h models.Heartbeat
		var statsJSON string
		if err := rows.Scan(
			&h.ID, &h.LicenseID, &h.NodeID, &h.ClientIP,
			&h.FingerprintHash, &statsJSON, &h.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("db: list heartbeats scan: %w", err)
		}
		if err := json.Unmarshal([]byte(statsJSON), &h.Stats); err != nil {
			h.Stats = map[string]any{}
		}
		heartbeats = append(heartbeats, h)
	}
	return heartbeats, rows.Err()
}

// ListHeartbeatsByLicense returns heartbeats for a specific license_id.
func (d *DB) ListHeartbeatsByLicense(licenseID string, limit int) ([]models.Heartbeat, error) {
	query := `SELECT id, license_id, node_id, client_ip, fingerprint_hash, stats_json, created_at
		FROM heartbeats WHERE license_id = ? ORDER BY created_at DESC LIMIT ?`

	rows, err := d.conn.Query(query, licenseID, limit)
	if err != nil {
		return nil, fmt.Errorf("db: list heartbeats by license: %w", err)
	}
	defer rows.Close()

	var heartbeats []models.Heartbeat
	for rows.Next() {
		var h models.Heartbeat
		var statsJSON string
		if err := rows.Scan(
			&h.ID, &h.LicenseID, &h.NodeID, &h.ClientIP,
			&h.FingerprintHash, &statsJSON, &h.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("db: list heartbeats by license scan: %w", err)
		}
		if err := json.Unmarshal([]byte(statsJSON), &h.Stats); err != nil {
			h.Stats = map[string]any{}
		}
		heartbeats = append(heartbeats, h)
	}
	return heartbeats, rows.Err()
}

// CountRecentHeartbeats returns the number of heartbeats in the last duration.
func (d *DB) CountRecentHeartbeats(since time.Time) (int, error) {
	query := `SELECT COUNT(*) FROM heartbeats WHERE created_at >= ?`
	var count int
	err := d.conn.QueryRow(query, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("db: count recent heartbeats: %w", err)
	}
	return count, nil
}
