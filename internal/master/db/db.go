// Package db cung cấp lớp truy cập database SQLite cho Master_Server.
// Sử dụng WAL mode, busy_timeout, và các PRAGMA tối ưu cho concurrent access.
//
// Xem sqlite.go cho khởi tạo DB, migrate.go cho schema,
// license.go, release.go, heartbeat.go, admin.go cho CRUD operations.
package db
