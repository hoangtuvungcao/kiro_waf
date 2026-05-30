package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"kiro_waf/internal/master/models"
)

// GetLatestReleaseCtx returns the most recent release for a given component and channel
// using the provided context.
func (d *DB) GetLatestReleaseCtx(ctx context.Context, component, channel string) (*models.Release, error) {
	query := `SELECT id, component, channel, version, artifact_url, sha256, notes, min_version, created_at
		FROM releases WHERE component = ? AND channel = ?
		ORDER BY created_at DESC LIMIT 1`

	r := &models.Release{}
	err := d.conn.QueryRowContext(ctx, query, component, channel).Scan(
		&r.ID, &r.Component, &r.Channel, &r.Version,
		&r.ArtifactURL, &r.SHA256, &r.Notes, &r.MinVersion, &r.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("db: get latest release: %w", err)
	}
	return r, nil
}

// GetReleasePointsCtx returns all releases with version and creation date for chart display
// using the provided context.
func (d *DB) GetReleasePointsCtx(ctx context.Context) ([]ReleasePoint, error) {
	query := `SELECT version, created_at FROM releases ORDER BY created_at ASC`

	rows, err := d.conn.QueryContext(ctx, query)
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
