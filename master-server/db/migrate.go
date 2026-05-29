package db

// migrate creates all tables and indexes if they don't exist.
func (d *DB) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS licenses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    license_id TEXT UNIQUE NOT NULL,
    license_key TEXT UNIQUE NOT NULL,
    customer_id TEXT NOT NULL,
    customer_name TEXT DEFAULT '',
    client_ip TEXT DEFAULT '',
    fingerprint_hash TEXT DEFAULT '',
    plan TEXT NOT NULL DEFAULT 'community',
    status TEXT NOT NULL DEFAULT 'active',
    valid_days INTEGER NOT NULL DEFAULT 365,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL,
    last_heartbeat_at DATETIME,
    notes TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS releases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    component TEXT NOT NULL,
    channel TEXT NOT NULL DEFAULT 'stable',
    version TEXT NOT NULL,
    artifact_url TEXT NOT NULL,
    sha256 TEXT NOT NULL,
    notes TEXT DEFAULT '',
    min_version TEXT DEFAULT '0.0.0',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(component, channel, version)
);

CREATE TABLE IF NOT EXISTS heartbeats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    license_id TEXT NOT NULL,
    node_id TEXT NOT NULL,
    client_ip TEXT NOT NULL,
    fingerprint_hash TEXT DEFAULT '',
    stats_json TEXT DEFAULT '{}',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS admin_login_attempts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ip TEXT NOT NULL,
    success INTEGER NOT NULL DEFAULT 0,
    attempted_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS admin_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_token TEXT UNIQUE NOT NULL,
    ip TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_licenses_key ON licenses(license_key);
CREATE INDEX IF NOT EXISTS idx_licenses_status ON licenses(status);
CREATE INDEX IF NOT EXISTS idx_heartbeats_license ON heartbeats(license_id);
CREATE INDEX IF NOT EXISTS idx_heartbeats_created ON heartbeats(created_at);
CREATE INDEX IF NOT EXISTS idx_releases_component_channel ON releases(component, channel);
CREATE INDEX IF NOT EXISTS idx_admin_attempts_ip ON admin_login_attempts(ip, attempted_at);
CREATE INDEX IF NOT EXISTS idx_admin_sessions_token ON admin_sessions(session_token);
`
	_, err := d.conn.Exec(schema)
	return err
}
