package status

import (
	"testing"
)

// TestStatusPackageExists verifies the status package is importable and compiles.
// The actual status logic is tested via the CLI binary in cmd/kiro-cli/main_test.go
// since the status command is implemented directly in main.go using the diagnostics package.
func TestStatusPackageExists(t *testing.T) {
	// This test ensures the package compiles correctly.
	// The status command logic is in cmd/kiro-cli/main.go (runStatus function)
	// and delegates to internal/shared/diagnostics.BuildStatus.
	t.Log("status package compiles successfully")
}
