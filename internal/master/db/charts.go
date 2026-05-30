package db

import (
	"fmt"
	"time"
)

// HourlyCount represents the number of heartbeats in a given hour.
type HourlyCount struct {
	Hour  string `json:"hour"`
	Count int    `json:"count"`
}

// ReleasePoint represents a release version with its creation timestamp.
type ReleasePoint struct {
	Version   string `json:"version"`
	CreatedAt string `json:"created_at"`
}

// GetHeartbeatTimeline returns hourly heartbeat counts for the last 24 hours.
// Returns an empty slice (not nil) when no data is available.
func (d *DB) GetHeartbeatTimeline() ([]HourlyCount, error) {
	since := time.Now().UTC().Add(-24 * time.Hour)

	query := `SELECT strftime('%Y-%m-%dT%H:00:00Z', created_at) AS hour, COUNT(*) AS count
		FROM heartbeats
		WHERE created_at >= ?
		GROUP BY hour
		ORDER BY hour ASC`

	rows, err := d.conn.Query(query, since)
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

// GetReleasePoints returns all releases with version and creation date for chart display.
// Returns an empty slice (not nil) when no data is available.
func (d *DB) GetReleasePoints() ([]ReleasePoint, error) {
	query := `SELECT version, created_at FROM releases ORDER BY created_at ASC`

	rows, err := d.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("db: get release points: %w", err)
	}
	defer rows.Close()

	var releases []ReleasePoint
	for rows.Next() {
		var rp ReleasePoint
		var createdAt time.Time
		if err := rows.Scan(&rp.Version, &createdAt); err != nil {
			return nil, fmt.Errorf("db: get release points scan: %w", err)
		}
		rp.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		releases = append(releases, rp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: get release points rows: %w", err)
	}

	// Return empty slice instead of nil for consistent JSON serialization.
	if releases == nil {
		releases = []ReleasePoint{}
	}
	return releases, nil
}
