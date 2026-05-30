package db

import (
	"database/sql"
	"fmt"
	"time"

	"kiro_waf/internal/master/models"
)

// LogLoginAttempt records an admin login attempt.
func (d *DB) LogLoginAttempt(ip string, success bool) error {
	successInt := 0
	if success {
		successInt = 1
	}
	query := `INSERT INTO admin_login_attempts (ip, success, attempted_at) VALUES (?, ?, ?)`
	_, err := d.conn.Exec(query, ip, successInt, time.Now())
	if err != nil {
		return fmt.Errorf("db: log login attempt: %w", err)
	}
	return nil
}

// CountFailedAttempts returns the number of failed login attempts from an IP since the given time.
func (d *DB) CountFailedAttempts(ip string, since time.Time) (int, error) {
	query := `SELECT COUNT(*) FROM admin_login_attempts WHERE ip = ? AND success = 0 AND attempted_at >= ?`
	var count int
	err := d.conn.QueryRow(query, ip, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("db: count failed attempts: %w", err)
	}
	return count, nil
}

// CreateSession inserts a new admin session.
func (d *DB) CreateSession(s *models.AdminSession) error {
	query := `INSERT INTO admin_sessions (session_token, ip, created_at, expires_at) VALUES (?, ?, ?, ?)`
	result, err := d.conn.Exec(query, s.SessionToken, s.IP, s.CreatedAt, s.ExpiresAt)
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

// GetSessionByToken retrieves a session by its token. Returns nil if not found or expired.
func (d *DB) GetSessionByToken(token string) (*models.AdminSession, error) {
	query := `SELECT id, session_token, ip, created_at, expires_at
		FROM admin_sessions WHERE session_token = ? AND expires_at > ?`

	s := &models.AdminSession{}
	err := d.conn.QueryRow(query, token, time.Now()).Scan(
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

// DeleteSession removes a session by its token.
func (d *DB) DeleteSession(token string) error {
	_, err := d.conn.Exec("DELETE FROM admin_sessions WHERE session_token = ?", token)
	if err != nil {
		return fmt.Errorf("db: delete session: %w", err)
	}
	return nil
}

// CleanExpiredSessions removes all expired sessions.
func (d *DB) CleanExpiredSessions() error {
	_, err := d.conn.Exec("DELETE FROM admin_sessions WHERE expires_at <= ?", time.Now())
	if err != nil {
		return fmt.Errorf("db: clean expired sessions: %w", err)
	}
	return nil
}
