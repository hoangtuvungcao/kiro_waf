package db

import (
	"context"
	"encoding/json"
	"fmt"

	"kiro_waf/internal/master/models"
)

// LogHeartbeatCtx inserts a heartbeat record using the provided context.
// If the context deadline is exceeded, returns a context.DeadlineExceeded error.
func (d *DB) LogHeartbeatCtx(ctx context.Context, h *models.Heartbeat) error {
	statsJSON, err := json.Marshal(h.Stats)
	if err != nil {
		statsJSON = []byte("{}")
	}

	query := `INSERT INTO heartbeats (license_id, node_id, client_ip, fingerprint_hash, stats_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`

	result, err := d.conn.ExecContext(ctx, query,
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

// GetHeartbeatTimelineCtx returns hourly heartbeat counts for the last 24 hours
// using the provided context.
func (d *DB) GetHeartbeatTimelineCtx(ctx context.Context) ([]HourlyCount, error) {
	query := `SELECT strftime('%Y-%m-%dT%H:00:00Z', created_at) AS hour, COUNT(*) AS count
		FROM heartbeats
		WHERE created_at >= datetime('now', '-24 hours')
		GROUP BY hour
		ORDER BY hour ASC`

	rows, err := d.conn.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("db: get heartbeat timeline: %w", err)
	}
	defer rows.Close()

	var timeline []HourlyCount
	for rows.Next() {
		var hc HourlyCount
		if err := rows.Scan(&hc.Hour, &hc.Count); err != nil {
			return nil, fmt.Errorf("db: get heartbeat timeline scan: %w", err)
		}
		timeline = append(timeline, hc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: get heartbeat timeline rows: %w", err)
	}

	// Return empty slice instead of nil for consistent JSON serialization.
	if timeline == nil {
		timeline = []HourlyCount{}
	}
	return timeline, nil
}
