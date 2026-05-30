package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"kiro_waf/internal/master/models"
)

// CountFailedAttemptsCtx returns the number of failed login attempts from an IP
// since the given time, using the provided context.
func (d *DB) CountFailedAttemptsCtx(ctx context.Context, ip string, since time.Time) (int, error) {
	query := `SELECT COUNT(*) FROM admin_login_attempts WHERE ip = ? AND success = 0 AND attempted_at >= ?`
	var count int
	err := d.conn.QueryRowContext(ctx, query, ip, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("db: count failed attempts: %w", err)
	}
	return count, nil
}

// GetSessionByTokenCtx retrieves a session by its token using the provided context.
// Returns nil if not found or expired.
func (d *DB) GetSessionByTokenCtx(ctx context.Context, token string) (*models.AdminSession, error) {
	query := `SELECT id, session_token, ip, created_at, expires_at
		FROM admin_sessions WHERE session_token = ? AND expires_at > ?`

	s := &models.AdminSession{}
	err := d.conn.QueryRowContext(ctx, query, token, time.Now()).Scan(
		&s.ID, &s.SessionToken, &s.IP, &s.CreatedAt, &s.ExpiresAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("db: get session by token: %w", err)
	}
	return s, nil
}

// LogLoginAttemptCtx records an admin login attempt using the provided context.
func (d *DB) LogLoginAttemptCtx(ctx context.Context, ip string, success bool) error {
	successInt := 0
	if success {
		successInt = 1
	}
	query := `INSERT INTO admin_login_attempts (ip, success, attempted_at) VALUES (?, ?, ?)`
	_, err := d.conn.ExecContext(ctx, query, ip, successInt, time.Now())
	if err != nil {
		return fmt.Errorf("db: log login attempt: %w", err)
	}
	return nil
}

// CreateSessionCtx inserts a new admin session using the provided context.
func (d *DB) CreateSessionCtx(ctx context.Context, s *models.AdminSession) error {
	query := `INSERT INTO admin_sessions (session_token, ip, created_at, expires_at) VALUES (?, ?, ?, ?)`
	result, err := d.conn.ExecContext(ctx, query, s.SessionToken, s.IP, s.CreatedAt, s.ExpiresAt)
	if err != nil {
		return fmt.Errorf("db: create session: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("db: create session last id: %w", err)
	}
	s.ID = id
	return nil
}

// DeleteSessionCtx removes a session by its token using the provided context.
func (d *DB) DeleteSessionCtx(ctx context.Context, token string) error {
	_, err := d.conn.ExecContext(ctx, "DELETE FROM admin_sessions WHERE session_token = ?", token)
	if err != nil {
		return fmt.Errorf("db: delete session: %w", err)
	}
	return nil
}
