//go:build tools

// Package tools tracks tool dependencies for the project.
// This file ensures go mod tidy retains required dependencies.
package tools

import (
	_ "github.com/mattn/go-sqlite3"
	_ "pgregory.net/rapid"
)
