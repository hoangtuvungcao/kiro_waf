package db

import (
	"database/sql"
	"fmt"

	"kiro_waf/master-server/models"
)

// CreateRelease inserts a new release into the database.
func (d *DB) CreateRelease(r *models.Release) error {
	query := `INSERT INTO releases (component, channel, version, artifact_url, sha256, notes, min_version, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := d.conn.Exec(query,
		r.Component, r.Channel, r.Version, r.ArtifactURL,
		r.SHA256, r.Notes, r.MinVersion, r.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("db: create release: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("db: create release last id: %w", err)
	}
	r.ID = id
	return nil
}

// GetReleaseByID retrieves a release by its database ID.
func (d *DB) GetReleaseByID(id int64) (*models.Release, error) {
	query := `SELECT id, component, channel, version, artifact_url, sha256, notes, min_version, created_at
		FROM releases WHERE id = ?`

	r := &models.Release{}
	err := d.conn.QueryRow(query, id).Scan(
		&r.ID, &r.Component, &r.Channel, &r.Version,
		&r.ArtifactURL, &r.SHA256, &r.Notes, &r.MinVersion, &r.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("db: get release by id: %w", err)
	}
	return r, nil
}

// GetLatestRelease returns the most recent release for a given component and channel.
func (d *DB) GetLatestRelease(component, channel string) (*models.Release, error) {
	query := `SELECT id, component, channel, version, artifact_url, sha256, notes, min_version, created_at
		FROM releases WHERE component = ? AND channel = ?
		ORDER BY created_at DESC LIMIT 1`

	r := &models.Release{}
	err := d.conn.QueryRow(query, component, channel).Scan(
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

// ListReleases returns all releases ordered by creation date descending.
func (d *DB) ListReleases() ([]models.Release, error) {
	query := `SELECT id, component, channel, version, artifact_url, sha256, notes, min_version, created_at
		FROM releases ORDER BY created_at DESC`

	rows, err := d.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("db: list releases: %w", err)
	}
	defer rows.Close()

	var releases []models.Release
	for rows.Next() {
		var r models.Release
		if err := rows.Scan(
			&r.ID, &r.Component, &r.Channel, &r.Version,
			&r.ArtifactURL, &r.SHA256, &r.Notes, &r.MinVersion, &r.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("db: list releases scan: %w", err)
		}
		releases = append(releases, r)
	}
	return releases, rows.Err()
}

// DeleteRelease removes a release by its database ID.
func (d *DB) DeleteRelease(id int64) error {
	result, err := d.conn.Exec("DELETE FROM releases WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("db: delete release: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("db: delete release: not found")
	}
	return nil
}
