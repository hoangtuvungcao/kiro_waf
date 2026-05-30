// Package db cung cấp lớp truy cập database SQLite cho Master_Server.
// Sử dụng WAL mode, busy_timeout, và các PRAGMA tối ưu cho concurrent access.
package db

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the sql.DB connection and provides CRUD operations.
type DB struct {
	conn *sql.DB
}

// New opens a SQLite database at the given path, applies PRAGMA settings,
// and runs schema migrations. Returns a ready-to-use DB instance.
func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("db: open %s: %w", dbPath, err)
	}

	// Apply PRAGMA settings for WAL mode and performance.
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA cache_size = -8000",
	}
	for _, p := range pragmas {
		if _, err := conn.Exec(p); err != nil {
			conn.Close()
			return nil, fmt.Errorf("db: pragma %q: %w", p, err)
		}
	}

	d := &DB{conn: conn}
	if err := d.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("db: migrate: %w", err)
	}

	return d, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.conn.Close()
}

// Conn returns the underlying *sql.DB for advanced usage.
func (d *DB) Conn() *sql.DB {
	return d.conn
}
